from .client import get_client

def HSet(key, field, value, *args):
    return get_client().send_command("HSET", key, field, value, *args)

def HGet(key, field):
    return get_client().send_command("HGET", key, field)

def HDel(key, *fields):
    return get_client().send_command("HDEL", key, *fields)

def HGetAll(key):
    return get_client().send_command("HGETALL", key)

def HIncrBy(key, field, increment):
    return get_client().send_command("HINCRBY", key, field, increment)

def HExists(key, field):
    return get_client().send_command("HEXISTS", key, field)

def HLen(key):
    return get_client().send_command("HLEN", key)

def HKeys(key):
    return get_client().send_command("HKEYS", key)

def HVals(key):
    return get_client().send_command("HVALS", key)

def HMSet(key, **mapping):
    args = [key]
    for k, v in mapping.items():
        args.extend([k, v])
    return get_client().send_command("HMSET", *args)

def HDelAll(key):
    return get_client().send_command("HDELALL", key)

def HExpire(key, field, seconds):
    return get_client().send_command("HEXPIRE", key, field, seconds)