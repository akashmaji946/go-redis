from .client import get_client

def Del(*keys):
    return get_client().send_command("DEL", *keys)

def Exists(*keys):
    return get_client().send_command("EXISTS", *keys)

def Keys(pattern):
    return get_client().send_command("KEYS", pattern)

def Rename(key, newkey):
    return get_client().send_command("RENAME", key, newkey)

def Type(key):
    return get_client().send_command("TYPE", key)

def Expire(key, seconds):
    return get_client().send_command("EXPIRE", key, seconds)

def Ttl(key):
    return get_client().send_command("TTL", key)

def Persist(key):
    return get_client().send_command("PERSIST", key)