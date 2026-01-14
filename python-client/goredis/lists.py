from .client import get_client

def LPush(key, *values):
    return get_client().send_command("LPUSH", key, *values)

def RPush(key, *values):
    return get_client().send_command("RPUSH", key, *values)

def LPop(key):
    return get_client().send_command("LPOP", key)

def RPop(key):
    return get_client().send_command("RPOP", key)

def LRange(key, start, stop):
    return get_client().send_command("LRANGE", key, start, stop)

def LLen(key):
    return get_client().send_command("LLEN", key)

def LIndex(key, index):
    return get_client().send_command("LINDEX", key, index)

def LGet(key):
    return get_client().send_command("LGET", key)