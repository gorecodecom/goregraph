// Explicit Roslyn export. No project builds, analyzers, generators or user code run.
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;

record Module(string name, string[] files, string[]? references, string[]? dependencies, string[]? defines);
record Request(string root, string[] inputs, Module[] modules);
record Declaration(string usr, string name, string kind, string file, int line, int column, string owner, string module);
record Reference(string usr, string caller, string name, string file, int line, int column, string kind);
record Report(int schema_version, string language, string producer, Dictionary<string, string> inputs,
    string[] covered_files, List<Declaration> declarations, List<Reference> references);

static class Program
{
    static string Hash(string file) => Convert.ToHexString(SHA256.HashData(File.ReadAllBytes(file))).ToLowerInvariant();
    static string Identity(ISymbol symbol)
    {
        if (symbol is IMethodSymbol method) symbol = (method.ReducedFrom ?? method).OriginalDefinition;
        else symbol = symbol.OriginalDefinition;
        return (symbol.ContainingAssembly?.Identity.Name ?? "") + ":" +
            (symbol.GetDocumentationCommentId() ?? symbol.ToDisplayString(SymbolDisplayFormat.FullyQualifiedFormat));
    }
    static string SafeInput(string root, string relative)
    {
        string full = Path.GetFullPath(Path.Combine(root, relative));
        if (Path.IsPathRooted(relative) || !full.StartsWith(root + Path.DirectorySeparatorChar, StringComparison.Ordinal))
            throw new InvalidDataException("Input escapes root: " + relative);
        var info = new FileInfo(full);
        if (info.LinkTarget != null) throw new InvalidDataException("Symlink input is unsupported");
        for (var parent = info.Directory; parent != null && parent.FullName != root; parent = parent.Parent)
            if (parent.LinkTarget != null) throw new InvalidDataException("Symlink input directory is unsupported");
        return full;
    }
    static (string file, int line, int column) Position(SyntaxTree tree, SyntaxToken token)
    {
        var point = tree.GetLineSpan(token.Span).StartLinePosition;
        string text = tree.GetText().Lines[point.Line].ToString();
        return (tree.FilePath, point.Line + 1, Encoding.UTF8.GetByteCount(text[..point.Character]) + 1);
    }
    static (SyntaxToken token, string kind)? DeclaredToken(SyntaxNode node) => node switch
    {
        ClassDeclarationSyntax n => (n.Identifier, "class"),
        StructDeclarationSyntax n => (n.Identifier, "struct"),
        InterfaceDeclarationSyntax n => (n.Identifier, "interface"),
        RecordDeclarationSyntax n => (n.Identifier, "record"),
        EnumDeclarationSyntax n => (n.Identifier, "enum"),
        EnumMemberDeclarationSyntax n => (n.Identifier, "enum_case"),
        MethodDeclarationSyntax n => (n.Identifier, "method"),
        OperatorDeclarationSyntax n => (n.OperatorToken, "method"),
        ConversionOperatorDeclarationSyntax n => (n.ImplicitOrExplicitKeyword, "method"),
        LocalFunctionStatementSyntax n => (n.Identifier, "function"),
        ConstructorDeclarationSyntax n => (n.Identifier, "constructor"),
        PropertyDeclarationSyntax n => (n.Identifier, "property"),
        VariableDeclaratorSyntax n when n.Parent?.Parent is FieldDeclarationSyntax => (n.Identifier, "field"),
        _ => null
    };
    static SyntaxToken? ReferencedToken(SyntaxNode node) => node switch
    {
        InvocationExpressionSyntax n when n.Expression is MemberAccessExpressionSyntax m => m.Name.Identifier,
        InvocationExpressionSyntax n when n.Expression is MemberBindingExpressionSyntax m => m.Name.Identifier,
        InvocationExpressionSyntax n when n.Expression is SimpleNameSyntax s => s.Identifier,
        ObjectCreationExpressionSyntax n when n.Type is SimpleNameSyntax s => s.Identifier,
        ObjectCreationExpressionSyntax n when n.Type is QualifiedNameSyntax q => q.Right.Identifier,
        ImplicitObjectCreationExpressionSyntax n => n.NewKeyword,
        BinaryExpressionSyntax n => n.OperatorToken,
        PrefixUnaryExpressionSyntax n => n.OperatorToken,
        PostfixUnaryExpressionSyntax n => n.OperatorToken,
        _ => null
    };
    static bool IsCallBinding(SyntaxNode node, IMethodSymbol target) => node is InvocationExpressionSyntax or BaseObjectCreationExpressionSyntax ||
        target.MethodKind is MethodKind.UserDefinedOperator or MethodKind.Conversion;
    static void Export(Request request, string output)
    {
        string root = Path.GetFullPath(request.root).TrimEnd(Path.DirectorySeparatorChar);
        var inputs = request.inputs.ToDictionary(p => p.Replace('\\', '/'), p => Hash(SafeInput(root, p)));
        var modules = request.modules.ToDictionary(m => m.name);
        var compilations = new Dictionary<string, CSharpCompilation>();
        var visiting = new HashSet<string>();
        CSharpCompilation Compile(string name)
        {
            if (compilations.TryGetValue(name, out var known)) return known;
            if (!visiting.Add(name)) throw new InvalidDataException("Cyclic module dependencies");
            var module = modules[name];
            var parse = new CSharpParseOptions(LanguageVersion.Latest, preprocessorSymbols: module.defines ?? []);
            var trees = module.files.Select(p => {
                if (!inputs.ContainsKey(p)) throw new InvalidDataException("Source omitted from inputs: " + p);
                return CSharpSyntaxTree.ParseText(File.ReadAllText(SafeInput(root, p)), parse, p, Encoding.UTF8);
            });
            var references = new List<MetadataReference>();
            string[] paths = module.references ?? ((string?)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES") ?? "").Split(Path.PathSeparator);
            references.AddRange(paths.Where(p => p.Length > 0).Select(p => MetadataReference.CreateFromFile(Path.GetFullPath(p, root))));
            foreach (string dependency in module.dependencies ?? []) references.Add(Compile(dependency).ToMetadataReference());
            var compilation = CSharpCompilation.Create(name, trees, references,
                new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary, nullableContextOptions: NullableContextOptions.Enable));
            var errors = compilation.GetDiagnostics().Where(d => d.Severity == DiagnosticSeverity.Error).ToArray();
            if (errors.Length != 0) throw new InvalidDataException(string.Join("\n", errors.Select(d => d.ToString())));
            visiting.Remove(name);
            compilations.Add(name, compilation);
            return compilation;
        }
        var declarations = new List<Declaration>();
        var references = new List<Reference>();
        var covered = new HashSet<string>();
        foreach (string name in modules.Keys) Compile(name);
        foreach (var entry in compilations)
        foreach (var tree in entry.Value.SyntaxTrees)
        {
            if (!covered.Add(tree.FilePath)) throw new InvalidDataException("A source belongs to multiple selected modules");
            var model = entry.Value.GetSemanticModel(tree);
            foreach (var node in tree.GetRoot().DescendantNodes())
            {
                if (DeclaredToken(node) is { } declaration && model.GetDeclaredSymbol(node) is { } symbol)
                {
                    var position = Position(tree, declaration.token);
                    declarations.Add(new(Identity(symbol), declaration.token.Text, declaration.kind, position.file,
                        position.line, position.column, symbol.ContainingType?.ToDisplayString() ?? "", entry.Key));
                }
                if (node is TypeDeclarationSyntax { ParameterList: { } parameters } primary && model.GetDeclaredSymbol(primary) is INamedTypeSymbol primaryType)
                {
                    var parameterSymbols = parameters.Parameters.Select(parameter => model.GetDeclaredSymbol(parameter) as IParameterSymbol).ToArray();
                    foreach (var constructor in primaryType.InstanceConstructors)
                    {
                        if (constructor.Parameters.Length != parameterSymbols.Length || !constructor.Parameters.Zip(parameterSymbols).All(pair => pair.Second != null && SymbolEqualityComparer.Default.Equals(pair.First.Type, pair.Second.Type))) continue;
                        var position = Position(tree, primary.Identifier);
                        declarations.Add(new(Identity(constructor), primary.Identifier.Text, "constructor", position.file, position.line, position.column, primaryType.ToDisplayString(), entry.Key));
                    }
                    if (primary is RecordDeclarationSyntax)
                    foreach (var parameter in parameters.Parameters)
                    foreach (var property in primaryType.GetMembers(parameter.Identifier.ValueText).OfType<IPropertySymbol>())
                    {
                        var position = Position(tree, parameter.Identifier);
                        declarations.Add(new(Identity(property), parameter.Identifier.Text, "property", position.file, position.line, position.column, primaryType.ToDisplayString(), entry.Key));
                    }
                }
                if (ReferencedToken(node) is { } token && model.GetSymbolInfo(node).Symbol is IMethodSymbol target && IsCallBinding(node, target))
                {
                    ISymbol? caller = null;
                    foreach (var ancestor in node.Ancestors())
                        if (DeclaredToken(ancestor) is { } && model.GetDeclaredSymbol(ancestor) is { } owner) { caller = owner; break; }
                    var position = Position(tree, token);
                    references.Add(new(Identity(target), caller == null ? "" : Identity(caller), token.Text,
                        position.file, position.line, position.column, "calls_method_owner"));
                }
                else if (node is SimpleNameSyntax identifier && model.GetSymbolInfo(identifier).Symbol is { } referenced)
                {
                    var position = Position(tree, identifier.Identifier);
                    var caller = node.Ancestors().Select(ancestor => model.GetDeclaredSymbol(ancestor)).FirstOrDefault(s => s is IMethodSymbol or IPropertySymbol);
                    references.Add(new(Identity(referenced), caller == null ? "" : Identity(caller), identifier.Identifier.Text,
                        position.file, position.line, position.column, "uses_symbol"));
                }
            }
        }
        foreach (var input in inputs) if (Hash(SafeInput(root, input.Key)) != input.Value) throw new IOException("Input changed during export");
        var report = new Report(1, "csharp", "Roslyn " + typeof(CSharpCompilation).Assembly.GetName().Version,
            inputs, covered.Order().ToArray(), declarations, references);
        using var stream = new FileStream(output, FileMode.CreateNew, FileAccess.Write);
        JsonSerializer.Serialize(stream, report, new JsonSerializerOptions { WriteIndented = true });
    }
    static int Main(string[] args)
    {
        try {
            if (args.Length != 2 || !args[1].EndsWith(".goregraph-csharp.json", StringComparison.Ordinal)) throw new ArgumentException("Usage: Exporter REQUEST.json NEW.goregraph-csharp.json");
            var request = JsonSerializer.Deserialize<Request>(File.ReadAllText(args[0])) ?? throw new InvalidDataException("Invalid request");
            Export(request, args[1]); return 0;
        } catch (Exception error) { Console.Error.WriteLine(error.Message); return 1; }
    }
}
