import mmap
import struct

MAGIC = b"CWLTREC2"
VERSION = 2
HEADER_FMT = "<8sIIIQ8I8s"
HEADER_SIZE = 68
RECORD_METADATA_FMT = "<QQ64sB3x"
RECORD_METADATA_SIZE = 84
RECORD_SIZE = 24660
FRAME_SIZE = 12288
MASK_OFFSET = 80


def parse_header(buf: memoryview):
    if len(buf) < HEADER_SIZE:
        raise ValueError("truncated V8 header")
    fields = struct.unpack_from(HEADER_FMT, buf, 0)
    magic, version, record_count, record_size, seed, *rest = fields
    class_counts = rest[:8]
    reserved = rest[8]
    if magic != MAGIC:
        raise ValueError(f"invalid magic: {magic!r}")
    if version != VERSION:
        raise ValueError(f"unsupported version: {version}")
    if record_size != RECORD_SIZE:
        raise ValueError(f"invalid record size: {record_size}")
    return {
        "magic": magic,
        "version": version,
        "record_count": record_count,
        "record_size": record_size,
        "seed": seed,
        "class_counts": class_counts,
        "reserved": reserved,
    }


def parse_record(buf: memoryview, offset: int):
    if offset < HEADER_SIZE or offset + RECORD_SIZE > len(buf):
        raise ValueError("record offset out of bounds")
    tick_before, tick_after, action_bytes, mask = struct.unpack_from(
        RECORD_METADATA_FMT, buf, offset
    )
    frame_t_start = offset + RECORD_METADATA_SIZE
    frame_next_start = frame_t_start + FRAME_SIZE
    return {
        "tick_before": tick_before,
        "tick_after": tick_after,
        "action_raw": action_bytes,
        "mask": mask,
        "rgb_t": buf[frame_t_start:frame_next_start],
        "rgb_next": buf[frame_next_start:offset + RECORD_SIZE],
    }


def record_offset(index: int) -> int:
    if index < 0:
        raise ValueError("negative record index")
    return HEADER_SIZE + index * RECORD_SIZE
