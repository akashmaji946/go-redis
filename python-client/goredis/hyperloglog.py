from .client import get_client

def PfAdd(key, *elements):
    """Add one or more elements to a HyperLogLog stored at key."""
    return get_client().send_command("PFADD", key, *elements)

def PfCount(*keys):
    """Return the approximated cardinality of the HyperLogLog(s) at key(s)."""
    return get_client().send_command("PFCOUNT", *keys)

def PfDebug(key):
    """Return internal debugging information about a HyperLogLog stored at key."""
    return get_client().send_command("PFDEBUG", key)

def PfMerge(dest_key, *source_keys):
    """Merge multiple HyperLogLog values into a single destination HyperLogLog."""
    return get_client().send_command("PFMERGE", dest_key, *source_keys)
