from .client import get_client

def HSet(key, field, value, *args):
    """
    Set the string value of a hash field.
    If the field already exists, it updates the value.
    """
    return get_client().send_command("HSET", key, field, value, *args)

def HGet(key, field):
    """Get the value of a hash field."""
    return get_client().send_command("HGET", key, field)

def HDel(key, *fields):
    """Delete one or more hash fields."""
    return get_client().send_command("HDEL", key, *fields)

def HGetAll(key):
    """Get all fields and values in a hash."""
    return get_client().send_command("HGETALL", key)

def HIncrBy(key, field, increment):
    """Increment the integer value of a hash field by the given amount."""
    return get_client().send_command("HINCRBY", key, field, increment)

def HExists(key, field):
    """Check if a hash field exists."""
    return get_client().send_command("HEXISTS", key, field)

def HLen(key):
    """Get the number of fields in a hash."""
    return get_client().send_command("HLEN", key)

def HKeys(key):
    """Get all field names in a hash."""
    return get_client().send_command("HKEYS", key)

def HVals(key):
    """Get all values in a hash."""
    return get_client().send_command("HVALS", key)

def HMSet(key, **mapping):
    """
    Set multiple hash fields to multiple values using keyword arguments.
    """
    args = [key]
    for k, v in mapping.items():
        args.extend([k, v])
    return get_client().send_command("HMSET", *args)

def HMGet(key, *fields):
    """Get the values of all the given hash fields."""
    return get_client().send_command("HMGET", key, *fields)

def HDelAll(key):
    """Delete all fields in a hash."""
    return get_client().send_command("HDELALL", key)

def HExpire(key, field, seconds):
    """Set expiration for a hash field in seconds."""
    return get_client().send_command("HEXPIRE", key, field, seconds)