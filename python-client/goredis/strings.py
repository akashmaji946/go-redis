from .client import get_client

def Get(key):
    """Retrieve the string value of a key."""
    return get_client().send_command("GET", key)

def Set(key, value):
    """Set the string value of a key."""
    return get_client().send_command("SET", key, value)

def Incr(key):
    """Increment the integer value of a key by one."""
    return get_client().send_command("INCR", key)

def Decr(key):
    """Decrement the integer value of a key by one."""
    return get_client().send_command("DECR", key)

def IncrBy(key, increment):
    """Increment the integer value of a key by the given amount."""
    return get_client().send_command("INCRBY", key, increment)

def DecrBy(key, decrement):
    """Decrement the integer value of a key by the given amount."""
    return get_client().send_command("DECRBY", key, decrement)

def MGet(*keys):
    """Get the values of all the given keys."""
    return get_client().send_command("MGET", *keys)

def MSet(**mapping):
    """Set multiple keys to multiple values using a dictionary or keyword arguments."""
    args = []
    for k, v in mapping.items():
        args.extend([k, v])
    return get_client().send_command("MSET", *args)

def StrLen(key):
    """Get the length of the string value stored at key."""
    return get_client().send_command("STRLEN", key)