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


def reference(property_name, kind, value):
    return {"property": property_name, "target": asset_id(kind, value.name)}


def export_object(obj):
    record = {
        "id": asset_id("object", obj.name), "name": obj.name, "kind": obj.type.lower(),
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
    animation = obj.animation_data
    if animation:
        if animation.action:
            record["references"].append(reference("action", "action", animation.action))
        for track in animation.nla_tracks:
            for strip in track.strips:
                if strip.action:
                    record["references"].append(reference("nla:" + track.name, "action", strip.action))
        record["properties"]["drivers"] = [curve.data_path for curve in animation.drivers]
    return record


def evaluated_mesh(obj, depsgraph):
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
        return {"vertices": vertices, "triangles": triangles,
                "degenerate_polygons": degenerate,
                "bounds_min": finite(min(point[axis] for point in bounds) for axis in range(3)),
                "bounds_max": finite(max(point[axis] for point in bounds) for axis in range(3))}
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
    frames = sorted(set(int(value) for value in args.frames.split(",") if value.strip()))
    if len(frames) > 128:
        raise ValueError("At most 128 explicitly selected frames are supported")
    before = sha256_file(source)
    objects = sorted(bpy.data.objects, key=lambda item: item.name)
    selected = objects[:args.max_objects]
    records = [export_object(obj) for obj in selected]
    for mesh in sorted(bpy.data.meshes, key=lambda item: item.name):
        records.append({"id": asset_id("mesh", mesh.name), "name": mesh.name, "kind": "mesh",
                        "properties": {"vertices": len(mesh.vertices), "polygons": len(mesh.polygons),
                                       "shape_keys": [key.name for key in mesh.shape_keys.key_blocks]
                                       if mesh.shape_keys else []}, "references": []})
    for armature in sorted(bpy.data.armatures, key=lambda item: item.name):
        records.append({"id": asset_id("armature", armature.name), "name": armature.name,
                        "kind": "armature", "properties": {"bones": [
                            {"name": bone.name, "parent": bone.parent.name if bone.parent else None,
                             "head": finite(bone.head_local), "tail": finite(bone.tail_local),
                             "deform": bone.use_deform} for bone in armature.bones]}, "references": []})
        for bone in armature.bones:
            links = [{"property": "armature", "target": asset_id("armature", armature.name)}]
            if bone.parent:
                links.append({"property": "parent", "target": asset_id("bone", armature.name + "/" + bone.parent.name)})
            records.append({"id": asset_id("bone", armature.name + "/" + bone.name),
                            "name": bone.name, "kind": "bone",
                            "properties": {"bone_head": finite(bone.head_local),
                                           "bone_tail": finite(bone.tail_local),
                                           "deform": bone.use_deform}, "references": links})
    for material in sorted(bpy.data.materials, key=lambda item: item.name):
        records.append({"id": asset_id("material", material.name), "name": material.name,
                        "kind": "material", "properties": {"use_nodes": material.node_tree is not None,
                            "node_types": sorted(node.bl_idname for node in material.node_tree.nodes)
                            if material.node_tree else []}, "references": []})
    for action in sorted(bpy.data.actions, key=lambda item: item.name):
        records.append({"id": asset_id("action", action.name), "name": action.name, "kind": "action",
                        "properties": {"frame_range": finite(action.frame_range),
                                       "slots": [slot.identifier for slot in action.slots],
                                       "channels": action_channels(action)}, "references": []})
    scene = bpy.context.scene
    original_frame = scene.frame_current
    samples = []
    try:
        for frame in frames or [original_frame]:
            scene.frame_set(frame)
            depsgraph = bpy.context.evaluated_depsgraph_get()
            for obj in selected:
                if obj.type == "MESH" and obj.name in scene.objects:
                    samples.append({"object": asset_id("object", obj.name), "frame": frame,
                                    **evaluated_mesh(obj, depsgraph)})
                elif obj.type == "ARMATURE" and obj.name in scene.objects:
                    evaluated = obj.evaluated_get(depsgraph)
                    for bone in evaluated.pose.bones:
                        samples.append({"object": asset_id("bone", obj.data.name + "/" + bone.name),
                                        "frame": frame, "bone_head": finite(evaluated.matrix_world @ bone.head),
                                        "bone_tail": finite(evaluated.matrix_world @ bone.tail),
                                        "bone_matrix": [finite(row) for row in evaluated.matrix_world @ bone.matrix]})
    finally:
        scene.frame_set(original_frame)
    if sha256_file(source) != before:
        raise RuntimeError("Source blend changed during export")
    report = {"schema_version": 1, "engine": "blender", "producer_version": bpy.app.version_string,
              "source": relative, "source_sha256": before, "objects": records, "samples": samples,
              "limitations": ["Explicit frame samples only; no exhaustive animation or collision proof",
                               "Viewport dependency graph; render-only settings may differ",
                               "External libraries and textures are metadata only",
                               "Action channels are limited to 4096; keyframes to 2048 per channel"],
              "truncated": len(objects) > len(selected)}
    destination.parent.mkdir(parents=True, exist_ok=True)
    fd, staging = tempfile.mkstemp(prefix=".goregraph-export-", dir=destination.parent)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as output:
            json.dump(report, output, indent=2, ensure_ascii=False, allow_nan=False)
            output.write("\n")
        os.replace(staging, destination)
    finally:
        if os.path.exists(staging):
            os.unlink(staging)


if __name__ == "__main__":
    main()
