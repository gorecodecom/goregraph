package scan

import "testing"

func TestCSharpTypedPersistenceAndRegistrations(t *testing.T) {
	s := parseCSharpSource(FileRecord{Path: "App.cs"}, `using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.DependencyInjection;
interface IService {}
class Service : IService {}
class Database : DbContext {public DbSet<Service> Items {get; set;}}
class App {
 Database db;
 void Configure(IServiceCollection services) {services.AddScoped<IService, Service>();}
 void Save(Service entity) {db.Items.Add(entity); db.SaveChanges();}
}`)
	result := analyzeCSharpProject([]csharpSource{s})
	registrations, persistence := 0, 0
	for _, fact := range result.facts.Declarations {
		if fact.Kind == "registration" {
			registrations++
		}
		if fact.Kind == "persistence" {
			persistence++
		}
	}
	if registrations != 1 || persistence != 2 || len(result.capabilities) != 2 {
		t.Fatalf("declarations=%#v capabilities=%#v", result.facts.Declarations, result.capabilities)
	}
	implementations := 0
	for _, ref := range result.facts.References {
		if ref.Type == "registers_implementation" && ref.Resolution == SymbolResolutionExact {
			implementations++
		}
	}
	if implementations != 1 {
		t.Fatal(result.facts.References)
	}
	for _, edge := range result.graph.Edges {
		if edge.To.Method == "AddScoped" || edge.To.Method == "SaveChanges" {
			t.Fatalf("framework registration became an execution edge: %#v", edge)
		}
	}
}

func TestCSharpFrameworkNamesWithoutBindingStayUnknown(t *testing.T) {
	for _, body := range []string{
		`class DbContext {public void SaveChanges(){}} class App {DbContext db; void Save(){db.SaveChanges();}}`,
		`using Microsoft.EntityFrameworkCore; class DbContext {public void SaveChanges(){}} class App {DbContext db; void Save(){db.SaveChanges();}}`,
		`using Microsoft.Extensions.DependencyInjection; class IServiceCollection {} class App {void Configure(IServiceCollection services){services.AddScoped<Unknown>();}}`,
		"using Microsoft.EntityFrameworkCore;\n#if FEATURE\nclass App {DbContext db; void Save(){db.SaveChanges();}}\n#endif",
	} {
		result := analyzeCSharpProject([]csharpSource{parseCSharpSource(FileRecord{Path: "App.cs"}, body)})
		if len(result.capabilities) != 0 {
			t.Fatalf("unbound framework promoted: %#v", result.capabilities)
		}
		for _, fact := range result.facts.Declarations {
			if fact.Kind == "registration" {
				t.Fatal(fact)
			}
		}
	}
}

func TestCSharpTopLevelRegistrationsPreserveDeclaredClasses(t *testing.T) {
	s := parseCSharpSource(FileRecord{Path: "Program.cs"}, `using Microsoft.Extensions.DependencyInjection;
var services = new ServiceCollection();
services.AddSingleton<IService, Service>();
interface IService {}
class Service: IService {void Configure(IServiceCollection other){other.AddScoped<IService, Service>();}}
`)
	result := analyzeCSharpProject([]csharpSource{s})
	registrations := 0
	for _, fact := range result.facts.Declarations {
		if fact.Kind == "registration" {
			registrations++
		}
	}
	if registrations != 2 {
		t.Fatalf("top-level absent or class registered twice: %#v", result.facts.Declarations)
	}
}

func TestCSharpMinimalAPIRoutesAndBuilderServices(t *testing.T) {
	s := parseCSharpSource(FileRecord{Path: "Program.cs"}, `using Microsoft.AspNetCore.Builder;
using Microsoft.Extensions.DependencyInjection;
var builder = WebApplication.CreateBuilder(args);
builder.Services.AddScoped<IService, Service>();
var app = builder.Build();
app.MapGet("/items", () => "ok");
app.MapPost(dynamicRoute, () => "unknown");
interface IService {}
class Service: IService {}
`)
	result := analyzeCSharpProject([]csharpSource{s})
	if len(result.code.Routes) != 1 || result.code.Routes[0].Path != "/items" {
		t.Fatal(result.code.Routes)
	}
	registrations := 0
	for _, fact := range result.facts.Declarations {
		if fact.Kind == "registration" {
			registrations++
		}
	}
	if registrations != 1 {
		t.Fatal(result.facts.Declarations)
	}
}

func TestCSharpShadowHttpClientIsNotHTTPConsumer(t *testing.T) {
	s := parseCSharpSource(FileRecord{Path: "Fake.cs"}, `using System.Net.Http; class HttpClient {public void GetAsync(string value) {}} class Caller {HttpClient client; void Run(){client.GetAsync("/items");}}`)
	if result := analyzeCSharpProject([]csharpSource{s}); len(result.code.APIContracts) != 0 {
		t.Fatal(result.code.APIContracts)
	}
}
