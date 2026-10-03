// Explicit read-only evidence export. Place in Assets/Editor and invoke manually.
// Batch: -executeMethod GoreGraphAssetExporter.Export -goregraph-source Assets/Unit.prefab
// -goregraph-output Evidence/Unit.goregraph-unity.json
#if UNITY_EDITOR
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using UnityEditor;
using UnityEngine;
using Object = UnityEngine.Object;

public static class GoreGraphAssetExporter
{
    [Serializable] public sealed class Reference { public string property; public string target; }
    [Serializable] public sealed class Dependency { public string file; public string sha256; }
    [Serializable] public sealed class Properties
    {
        public string hierarchy;
        public bool active;
        public string[] components;
        public float[] position;
        public float[] scale;
        public int vertices;
        public long indices;
        public string[] bones;
        public string[] blend_shapes;
        public string[] animations;
        public float animation_length;
        public float[] bounds_min;
        public float[] bounds_max;
    }
    [Serializable] public sealed class Record
    {
        public string id; public string name; public string kind;
        public Properties properties = new Properties();
        public List<Reference> references = new List<Reference>();
    }
    [Serializable] public sealed class Report
    {
        public int schema_version = 1; public string engine = "unity";
        public string producer_version = Application.unityVersion;
        public string source; public string source_sha256;
        public List<Dependency> dependency_files = new List<Dependency>();
        public List<Record> objects = new List<Record>();
        public string[] limitations = { "Persistent imported asset metadata only; no scene or prefab instantiation",
            "No runtime, animation sampling or rendered geometry proof", "External references may remain unindexed" };
        public bool truncated;
    }

    private static string Argument(string name)
    {
        string[] args = Environment.GetCommandLineArgs();
        int at = Array.IndexOf(args, name);
        return at >= 0 && at + 1 < args.Length ? args[at + 1] : null;
    }
    private static string Identity(Object value)
    {
        return value == null ? null : GlobalObjectId.GetGlobalObjectIdSlow(value).ToString();
    }
    private static float[] Vector(Vector3 value) { return new[] { value.x, value.y, value.z }; }
    private static string Hash(string source)
    {
        using (var stream = File.OpenRead(source))
        using (var sha = SHA256.Create())
            return BitConverter.ToString(sha.ComputeHash(stream)).Replace("-", "").ToLowerInvariant();
    }
    private static string OwnedFile(string project, string relative)
    {
        string full = Path.GetFullPath(Path.Combine(project, relative));
        if (!full.StartsWith(project + Path.DirectorySeparatorChar, StringComparison.Ordinal))
            throw new ArgumentException("Dependency escapes the project");
        for (string current = full; current != project; current = Path.GetDirectoryName(current))
            if ((File.GetAttributes(current) & FileAttributes.ReparsePoint) != 0)
                throw new ArgumentException("Symlink dependencies are unsupported");
        return full;
    }
    private static void Link(Record record, string property, Object target)
    {
        if (target != null) record.references.Add(new Reference { property = property, target = Identity(target) });
    }
    private static void Add(Report report, Object value, HashSet<string> seen)
    {
        if (value == null || !EditorUtility.IsPersistent(value)) return;
        string id = Identity(value);
        if (!seen.Add(id)) return;
        if (report.objects.Count >= 100000) { report.truncated = true; return; }
        var record = new Record { id = id, name = value.name, kind = value.GetType().Name };
        if (value is GameObject gameObject)
        {
            record.properties.active = gameObject.activeSelf;
            record.properties.hierarchy = AnimationUtility.CalculateTransformPath(gameObject.transform, null);
            record.properties.components = gameObject.GetComponents<Component>().Select(component => component == null ? "MissingScript" : component.GetType().FullName).ToArray();
            Link(record, "transform", gameObject.transform);
        }
        if (value is Transform transform)
        {
            record.properties.position = Vector(transform.localPosition);
            record.properties.scale = Vector(transform.localScale);
            Link(record, "parent", transform.parent);
            Link(record, "gameObject", transform.gameObject);
        }
        if (value is Mesh mesh)
        {
            record.properties.vertices = mesh.vertexCount;
            for (int index = 0; index < mesh.subMeshCount; index++) record.properties.indices += mesh.GetIndexCount(index);
            record.properties.blend_shapes = Enumerable.Range(0, mesh.blendShapeCount).Select(mesh.GetBlendShapeName).ToArray();
            record.properties.bounds_min = Vector(mesh.bounds.min);
            record.properties.bounds_max = Vector(mesh.bounds.max);
        }
        if (value is SkinnedMeshRenderer skinned)
        {
            Link(record, "mesh", skinned.sharedMesh);
            record.properties.bones = skinned.bones.Select(bone => bone == null ? "MissingBone" : bone.name).ToArray();
            foreach (var bone in skinned.bones) Link(record, "bone", bone);
            Link(record, "rootBone", skinned.rootBone);
        }
        if (value is MeshFilter filter) Link(record, "mesh", filter.sharedMesh);
        if (value is Renderer renderer) foreach (var material in renderer.sharedMaterials) Link(record, "material", material);
        if (value is MonoBehaviour behaviour) Link(record, "script", MonoScript.FromMonoBehaviour(behaviour));
        if (value is Animator animator && animator.runtimeAnimatorController != null)
        {
            Link(record, "controller", animator.runtimeAnimatorController);
            record.properties.animations = animator.runtimeAnimatorController.animationClips.Select(clip => clip.name).ToArray();
        }
        if (value is AnimationClip clip) record.properties.animation_length = clip.length;
        // Serialized references are read without instantiating scene objects or invoking methods.
        var serialized = new SerializedObject(value);
        var iterator = serialized.GetIterator();
        while (iterator.Next(true))
            if (iterator.propertyType == SerializedPropertyType.ObjectReference)
                Link(record, iterator.propertyPath, iterator.objectReferenceValue);
        report.objects.Add(record);
    }
    public static void Export()
    {
        string project = Directory.GetParent(Application.dataPath).FullName;
        string source = Argument("-goregraph-source");
        string output = Argument("-goregraph-output");
        if (string.IsNullOrEmpty(source) || string.IsNullOrEmpty(output) || !output.EndsWith(".goregraph-unity.json", StringComparison.Ordinal))
            throw new ArgumentException("Source and .goregraph-unity.json output are required");
        string fullSource = Path.GetFullPath(Path.Combine(project, source));
        if (!fullSource.StartsWith(project + Path.DirectorySeparatorChar, StringComparison.Ordinal) || !source.StartsWith("Assets/", StringComparison.Ordinal))
            throw new ArgumentException("Source must be an asset inside this project");
        source = fullSource.Substring(project.Length + 1).Replace('\\', '/');
        fullSource = OwnedFile(project, source);
        string before = Hash(fullSource);
        var report = new Report { source = source.Replace('\\', '/'), source_sha256 = before };
        var dependencyPaths = new HashSet<string>(StringComparer.Ordinal) { source };
        foreach (string dependency in AssetDatabase.GetDependencies(source, true))
            if (dependency.StartsWith("Assets/", StringComparison.Ordinal)) dependencyPaths.Add(dependency);
        foreach (string configuration in new[] { "ProjectSettings/ProjectVersion.txt", "Packages/manifest.json", "Packages/packages-lock.json" })
            if (File.Exists(Path.Combine(project, configuration))) dependencyPaths.Add(configuration);
        foreach (string dependency in dependencyPaths.ToArray())
            if (File.Exists(Path.Combine(project, dependency + ".meta"))) dependencyPaths.Add(dependency + ".meta");
        if (dependencyPaths.Count > 10000) throw new IOException("Dependency inventory exceeds 10000 files");
        foreach (string dependency in dependencyPaths.OrderBy(path => path, StringComparer.Ordinal))
            report.dependency_files.Add(new Dependency { file = dependency.Replace('\\', '/'), sha256 = Hash(OwnedFile(project, dependency)) });
        string importedHash = AssetDatabase.GetAssetDependencyHash(source).ToString();
        var seen = new HashSet<string>();
        foreach (var value in AssetDatabase.LoadAllAssetsAtPath(source)) Add(report, value, seen);
        var root = AssetDatabase.LoadAssetAtPath<GameObject>(source);
        if (root != null)
            foreach (var transform in root.GetComponentsInChildren<Transform>(true))
            {
                Add(report, transform.gameObject, seen);
                foreach (var component in transform.GetComponents<Component>()) Add(report, component, seen);
            }
        if (Argument("-goregraph-include-dependencies") == "true")
            foreach (string dependency in dependencyPaths.OrderBy(path => path, StringComparer.Ordinal))
                if (dependency.StartsWith("Assets/", StringComparison.Ordinal) && !dependency.EndsWith(".meta", StringComparison.Ordinal))
                    foreach (var value in AssetDatabase.LoadAllAssetsAtPath(dependency)) Add(report, value, seen);
        if (Hash(fullSource) != before) throw new IOException("Source changed during export");
        foreach (var dependency in report.dependency_files)
            if (Hash(OwnedFile(project, dependency.file)) != dependency.sha256) throw new IOException("Dependency changed during export");
        if (AssetDatabase.GetAssetDependencyHash(source).ToString() != importedHash) throw new IOException("Imported asset dependencies changed during export");
        string destination = Path.GetFullPath(output);
        Directory.CreateDirectory(Path.GetDirectoryName(destination));
        string temporary = destination + "." + Guid.NewGuid().ToString("N") + ".tmp";
        try
        {
            string body = JsonUtility.ToJson(report, true) + "\n";
            if (System.Text.Encoding.UTF8.GetByteCount(body) > 16 * 1024 * 1024) throw new IOException("Export exceeds 16 MiB; reduce dependency selection");
            File.WriteAllText(temporary, body);
            if (File.Exists(destination)) File.Replace(temporary, destination, null);
            else File.Move(temporary, destination);
        }
        finally { if (File.Exists(temporary)) File.Delete(temporary); }
        Debug.Log("GoreGraph asset export written: " + destination);
    }
}
#endif
