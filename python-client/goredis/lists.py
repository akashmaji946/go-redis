from .client import get_client

def LPush(key, *values):
    """Prepend one or multiple values to a list."""
    return get_client().send_command("LPUSH", key, *values)

def RPush(key, *values):
    """Append one or multiple values to a list."""
    return get_client().send_command("RPUSH", key, *values)

def LPop(key):
    """Remove and get the first element in a list."""
    return get_client().send_command("LPOP", key)

def RPop(key):
    """Remove and get the last element in a list."""
    return get_client().send_command("RPOP", key)

def LRange(key, start, stop):
    """Get a range of elements from a list."""
    return get_client().send_command("LRANGE", key, start, stop)

def LLen(key):
    """Get the length of a list."""
    return get_client().send_command("LLEN", key)

def LIndex(key, index):
    """Get an element from a list by its index."""
    return get_client().send_command("LINDEX", key, index)

def LGet(key):
    """Get all elements in a list."""
    return get_client().send_command("LGET", key)