from .client import get_client

def Del(*keys):
    """Delete one or more keys."""
    return get_client().send_command("DEL", *keys)

def Delete(*keys):
    """Alias for Del."""
    return Del(*keys)

def Exists(*keys):
    """Check if keys exist."""
    return get_client().send_command("EXISTS", *keys)

def Keys(pattern):
    """Find all keys matching the given pattern."""
    return get_client().send_command("KEYS", pattern)

def Rename(key, newkey):
    """Rename a key."""
    return get_client().send_command("RENAME", key, newkey)

def Type(key):
    """Determine the type stored at key."""
    return get_client().send_command("TYPE", key)

def Expire(key, seconds):
    """Set a key's time to live in seconds."""
    return get_client().send_command("EXPIRE", key, seconds)

def Ttl(key):
    """Get the time to live for a key in seconds."""
    return get_client().send_command("TTL", key)

def Persist(key):
    """Remove the expiration from a key."""
    return get_client().send_command("PERSIST", key)