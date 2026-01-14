from .client import get_client

def Get(key):
    return get_client().send_command("GET", key)

def Set(key, value):
    return get_client().send_command("SET", key, value)

def Incr(key):
    return get_client().send_command("INCR", key)

def Decr(key):
    return get_client().send_command("DECR", key)

def IncrBy(key, increment):
    return get_client().send_command("INCRBY", key, increment)

def DecrBy(key, decrement):
    return get_client().send_command("DECRBY", key, decrement)

def MGet(*keys):
    return get_client().send_command("MGET", *keys)

def MSet(**mapping):
    args = []
    for k, v in mapping.items():
        args.extend([k, v])
    return get_client().send_command("MSET", *args)

def StrLen(key):
    return get_client().send_command("STRLEN", key)