"""Explicit, read-only Blender evidence export for GoreGraph.

Run with Blender --background --disable-autoexec PROJECT.blend --python SCRIPT
-- --root PROJECT_ROOT --output REPORT.goregraph-blender.json [--frames 1,10,20].
The exporter never saves the blend file and never runs project text scripts.
"""
import argparse
import hashlib
import json
import math
import os
import sys
import tempfile
from pathlib import Path

import bpy
from mathutils import Vector
from bpy_extras.anim_utils import action_get_channelbag_for_slot


def sha256_file(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(65536), b""):
            digest.update(block)
    return digest.hexdigest()


def finite(values):
    return [round(float(value), 6) if math.isfinite(float(value)) else None for value in values]


def asset_id(kind, name):
    return kind + ":" + name


def datablock_id(kind, value):
    name = value.name
    if value.library:
        name = value.library.filepath + "#" + name
    return asset_id(kind, name)


def reference(property_name, kind, value):
    return {"property": property_name, "target": datablock_id(kind, value)}


def driver_evidence(value):
    animation = getattr(value, "animation_data", None)
    drivers, links = [], []
    if not animation:
        return drivers, links
    kinds = {"Object": "object", "Material": "material", "Key": "shape_keys",
             "Scene": "scene", "Mesh": "mesh", "Armature": "armature"}
    for curve in list(animation.drivers)[:4096]:
        record = {"path": curve.data_path, "index": curve.array_index,
                  "type": curve.driver.type, "valid": curve.driver.is_valid, "variables": []}
        for variable in curve.driver.variables:
            targets = []
            for target in variable.targets:
                target_id = target.id
                targets.append({"path": target.data_path, "bone": target.bone_target,
                                "transform_type": target.transform_type})
                kind = kinds.get(target_id.bl_rna.identifier) if target_id else None
                if kind:
                    links.append(reference("driver:" + curve.data_path + "/" + variable.name, kind, target_id))
            record["variables"].append({"name": variable.name, "type": variable.type, "targets": targets})
        drivers.append(record)
    return drivers, links


def node_evidence(tree):
    if tree is None:
        return {}, []
    links = []
    for node in tree.nodes:
        for attribute, kind in [("image", "image"), ("node_tree", "node_group")]:
            value = getattr(node, attribute, None)
            if value:
                links.append(reference("node:" + node.name + "/" + attribute, kind, value))
        for socket in node.inputs:
            value = getattr(socket, "default_value", None)
            if isinstance(value, bpy.types.Object):
                links.append(reference("node:" + node.name + "/" + socket.name, "object", value))
    return {"node_types": sorted(node.bl_idname for node in tree.nodes),
            "node_links": [{"from_node": link.from_node.name, "from_socket": link.from_socket.name,
                            "to_node": link.to_node.name, "to_socket": link.to_socket.name}
                           for link in list(tree.links)[:8192]]}, links


def export_object(obj):
    record = {
        "id": datablock_id("object", obj), "name": obj.name, "kind": obj.type.lower(),
        "properties": {"location": finite(obj.location), "scale": finite(obj.scale),
                       "rotation_mode": obj.rotation_mode, "hidden_render": obj.hide_render,
                       "modifiers": [{"name": item.name, "type": item.type,
                                      "render": item.show_render, "viewport": item.show_viewport}
                                     for item in obj.modifiers]}, "references": []}
    if obj.parent:
        record["references"].append(reference("parent", "object", obj.parent))
    if obj.data and obj.type in {"MESH", "ARMATURE"}:
        record["references"].append(reference("data", obj.type.lower(), obj.data))
    for material in obj.material_slots:
        if material.material:
            record["references"].append(reference("material", "material", material.material))
    for constraint in obj.constraints:
        target = getattr(constraint, "target", None)
        if target:
            record["references"].append(reference("constraint:" + constraint.name, "object", target))
    for modifier in obj.modifiers:
        target = getattr(modifier, "object", None)
        if target:
            record["references"].append(reference("modifier:" + modifier.name, "object", target))
        group = getattr(modifier, "node_group", None)
        if group:
            record["references"].append(reference("modifier:" + modifier.name, "node_group", group))
    if obj.pose:
        for bone in obj.pose.bones:
            for constraint in bone.constraints:
                target = getattr(constraint, "target", None)
                if target:
                    record["references"].append(reference("pose:" + bone.name + "/" + constraint.name, "object", target))
    drivers, links = driver_evidence(obj)
    record["properties"]["drivers"] = drivers
    record["references"].extend(links)
    animation = obj.animation_data
    if animation:
        if animation.action:
            record["references"].append(reference("action", "action", animation.action))
        for track in animation.nla_tracks:
            for strip in track.strips:
                if strip.action:
                    record["references"].append(reference("nla:" + track.name, "action", strip.action))
    return record


def evaluated_mesh(obj, depsgraph, geometry_budget=None):
    evaluated = obj.evaluated_get(depsgraph)
    mesh = evaluated.to_mesh()
    if mesh is None:
        return {}
    try:
        mesh.calc_loop_triangles()
        vertices = len(mesh.vertices)
        triangles = len(mesh.loop_triangles)
        degenerate = sum(1 for polygon in mesh.polygons if polygon.area <= 1e-12)
        bounds = [evaluated.matrix_world @ Vector(corner) for corner in evaluated.bound_box]
        result = {"vertices": vertices, "triangles": triangles,
                "degenerate_polygons": degenerate,
                "bounds_min": finite(min(point[axis] for point in bounds) for axis in range(3)),
                "bounds_max": finite(max(point[axis] for point in bounds) for axis in range(3))}
        if geometry_budget is not None:
            complete = vertices <= geometry_budget[0] and triangles <= geometry_budget[1]
            result["geometry_complete"] = complete
            result["geometry_space"] = "world"
            if complete:
                result["vertex_positions"] = [finite(evaluated.matrix_world @ vertex.co) for vertex in mesh.vertices]
                result["triangle_indices"] = [list(triangle.vertices) for triangle in mesh.loop_triangles]
                geometry_budget[0] -= vertices
                geometry_budget[1] -= triangles
        return result
    finally:
        evaluated.to_mesh_clear()


def action_channels(action):
    channels = []
    for slot in action.slots:
        bag = action_get_channelbag_for_slot(action, slot)
        if bag is None:
            continue
        for curve in bag.fcurves:
            channels.append({"slot": slot.identifier, "path": curve.data_path,
                             "index": curve.array_index,
                             "keyframes": [{"frame_value": finite(point.co),
                                            "interpolation": point.interpolation}
                                           for point in list(curve.keyframe_points)[:2048]],
                             "keyframes_truncated": len(curve.keyframe_points) > 2048})
    return channels[:4096]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--frames", default="")
    parser.add_argument("--max-objects", type=int, default=4096)
    parser.add_argument("--geometry", action="store_true", help="Include bounded evaluated world-space surfaces for selected frames")
    parser.add_argument("--max-geometry-vertices", type=int, default=20000)
    parser.add_argument("--max-geometry-triangles", type=int, default=40000)
    args = parser.parse_args(sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else [])
    if bpy.context.preferences.filepaths.use_scripts_auto_execute:
        raise RuntimeError("Use --disable-autoexec; project scripts must remain disabled")
    root = Path(args.root).resolve(strict=True)
    source = Path(bpy.data.filepath).resolve(strict=True)
    relative = source.relative_to(root).as_posix()
    destination = Path(args.output).resolve()
    if not destination.name.endswith(".goregraph-blender.json"):
        raise ValueError("Output must end with .goregraph-blender.json")
    if not 1 <= args.max_objects <= 100000:
        raise ValueError("max-objects must be between 1 and 100000")
    if not 1 <= args.max_geometry_vertices <= 100000 or not 1 <= args.max_geometry_triangles <= 200000:
        raise ValueError("Geometry budgets exceed supported limits")
    frames = sorted(set(int(value) for value in args.frames.split(",") if value.strip()))
    if len(frames) > 128:
        raise ValueError("At most 128 explicitly selected frames are supported")
    before = sha256_file(source)
    dependencies = {}
    external_libraries = False
    for library in bpy.data.libraries:
        library_path = Path(bpy.path.abspath(library.filepath, library=library.parent)).resolve(strict=True)
        try:
            relative_library = library_path.relative_to(root).as_posix()
        except ValueError:
            external_libraries = True
            continue
        dependencies[relative_library] = sha256_file(library_path)
    objects = sorted(bpy.data.objects, key=lambda item: item.name)
    selected = objects[:args.max_objects]
    records = [export_object(obj) for obj in selected]
    for mesh in sorted(bpy.data.meshes, key=lambda item: item.name):
        links = [reference("shape_keys", "shape_keys", mesh.shape_keys)] if mesh.shape_keys else []
        records.append({"id": datablock_id("mesh", mesh), "name": mesh.name, "kind": "mesh",
                        "properties": {"vertices": len(mesh.vertices), "polygons": len(mesh.polygons),
                                       "shape_keys": [key.name for key in mesh.shape_keys.key_blocks]
                                       if mesh.shape_keys else []}, "references": links})
    for armature in sorted(bpy.data.armatures, key=lambda item: item.name):
        records.append({"id": datablock_id("armature", armature), "name": armature.name,
                        "kind": "armature", "properties": {"bones": [
                            {"name": bone.name, "parent": bone.parent.name if bone.parent else None,
                             "head": finite(bone.head_local), "tail": finite(bone.tail_local),
                             "deform": bone.use_deform} for bone in armature.bones]}, "references": []})
        for bone in armature.bones:
            bone_prefix = datablock_id("armature", armature)
            links = [{"property": "armature", "target": bone_prefix}]
            if bone.parent:
                links.append({"property": "parent", "target": asset_id("bone", bone_prefix + "/" + bone.parent.name)})
            records.append({"id": asset_id("bone", bone_prefix + "/" + bone.name),
                            "name": bone.name, "kind": "bone",
                            "properties": {"bone_head": finite(bone.head_local),
                                           "bone_tail": finite(bone.tail_local),
                                           "deform": bone.use_deform}, "references": links})
    for material in sorted(bpy.data.materials, key=lambda item: item.name):
        properties, links = node_evidence(material.node_tree)
        properties["use_nodes"] = material.node_tree is not None
        records.append({"id": datablock_id("material", material), "name": material.name,
                        "kind": "material", "properties": properties, "references": links})
    for group in sorted(bpy.data.node_groups, key=lambda item: item.name):
        properties, links = node_evidence(group)
        records.append({"id": datablock_id("node_group", group), "name": group.name,
                        "kind": "node_group", "properties": properties, "references": links})
    for image in sorted(bpy.data.images, key=lambda item: item.name):
        records.append({"id": datablock_id("image", image), "name": image.name, "kind": "image",
                        "properties": {"packed": image.packed_file is not None,
                                       "image_size": list(image.size)}, "references": []})
    for keys in sorted(bpy.data.shape_keys, key=lambda item: item.name):
        drivers, links = driver_evidence(keys)
        records.append({"id": datablock_id("shape_keys", keys), "name": keys.name, "kind": "shape_keys",
                        "properties": {"drivers": drivers, "shape_keys": [{"name": key.name,
                            "value": key.value, "min": key.slider_min, "max": key.slider_max}
                            for key in keys.key_blocks]}, "references": links})
    for collection in sorted(bpy.data.collections, key=lambda item: item.name):
        links = [reference("contains_object", "object", obj) for obj in collection.objects]
        links.extend(reference("child_collection", "collection", child) for child in collection.children)
        records.append({"id": datablock_id("collection", collection), "name": collection.name,
                        "kind": "collection", "properties": {"hidden_render": collection.hide_render}, "references": links})
    for item in sorted(bpy.data.scenes, key=lambda item: item.name):
        links = [reference("scene_object", "object", obj) for obj in item.objects]
        properties, driver_links = driver_evidence(item)
        links.extend(driver_links)
        records.append({"id": datablock_id("scene", item), "name": item.name, "kind": "scene",
                        "properties": {"drivers": properties, "frame_range": [item.frame_start,item.frame_end]}, "references": links})
    for action in sorted(bpy.data.actions, key=lambda item: item.name):
        records.append({"id": datablock_id("action", action), "name": action.name, "kind": "action",
                        "properties": {"frame_range": finite(action.frame_range),
                                       "slots": [slot.identifier for slot in action.slots],
                                       "channels": action_channels(action)}, "references": []})
    scene = bpy.context.scene
    original_frame = scene.frame_current
    samples = []
    geometry_budget = [args.max_geometry_vertices, args.max_geometry_triangles] if args.geometry else None
    try:
        for frame in frames or [original_frame]:
            scene.frame_set(frame)
            depsgraph = bpy.context.evaluated_depsgraph_get()
            for obj in selected:
                if obj.type == "MESH" and obj.name in scene.objects:
                    samples.append({"object": datablock_id("object", obj), "frame": frame,
                                    **evaluated_mesh(obj, depsgraph, geometry_budget)})
                elif obj.type == "ARMATURE" and obj.name in scene.objects:
                    evaluated = obj.evaluated_get(depsgraph)
                    for bone in evaluated.pose.bones:
                        pose_id = asset_id("pose_bone", datablock_id("object", obj) + "/" + bone.name)
                        if frame == (frames or [original_frame])[0]:
                            records.append({"id": pose_id, "name": bone.name, "kind": "pose_bone", "properties": {},
                                            "references": [{"property": "definition", "target": asset_id("bone", datablock_id("armature", obj.data) + "/" + bone.name)},
                                                           reference("instance", "object", obj)]})
                        samples.append({"object": pose_id,
                                        "frame": frame, "bone_head": finite(evaluated.matrix_world @ bone.head),
                                        "bone_tail": finite(evaluated.matrix_world @ bone.tail),
                                        "bone_matrix": [finite(row) for row in evaluated.matrix_world @ bone.matrix]})
    finally:
        scene.frame_set(original_frame)
    if sha256_file(source) != before:
        raise RuntimeError("Source blend changed during export")
    if any(sha256_file(root / name) != value for name, value in dependencies.items()):
        raise RuntimeError("Linked blend dependency changed during export")
    truncated = len(objects) > len(selected) or len(records) > 100000
    records = records[:100000]
    report = {"schema_version": 1, "engine": "blender", "producer_version": bpy.app.version_string,
              "source": relative, "source_sha256": before, "objects": records, "samples": samples,
              "dependencies": dependencies,
              "limitations": ["Explicit frame samples only; no exhaustive animation or collision proof",
                               "Viewport dependency graph; render-only settings may differ",
                               "Unindexed external libraries and textures are not freshness-verified",
                               "Action channels are limited to 4096; keyframes to 2048 per channel"],
              "truncated": truncated}
    if external_libraries:
        report["limitations"].append("Evaluated samples depend on external linked libraries outside the owned root")
    if args.geometry:
        report["limitations"].append("Surface coordinates cover only samples marked geometry_complete; intersections are not automatically proven")
    encoded = json.dumps(report, indent=2, ensure_ascii=False, allow_nan=False) + "\n"
    if len(encoded.encode("utf-8")) > 16 * 1024 * 1024:
        raise ValueError("Export exceeds 16 MiB; reduce object, frame or geometry limits")
    destination.parent.mkdir(parents=True, exist_ok=True)
    fd, staging = tempfile.mkstemp(prefix=".goregraph-export-", dir=destination.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as output:
            output.write(encoded)
        os.replace(staging, destination)
    finally:
        if os.path.exists(staging):
            os.unlink(staging)


if __name__ == "__main__":
    main()
