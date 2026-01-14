from .client import get_client

def SAdd(key, *members):
    return get_client().send_command("SADD", key, *members)

def SRem(key, *members):
    return get_client().send_command("SREM", key, *members)

def SMembers(key):
    return get_client().send_command("SMEMBERS", key)

def SIsMember(key, member):
    return get_client().send_command("SISMEMBER", key, member)

def SCard(key):
    return get_client().send_command("SCARD", key)

def SDiff(*keys):
    return get_client().send_command("SDIFF", *keys)

def SInter(*keys):
    return get_client().send_command("SINTER", *keys)

def SUnion(*keys):
    return get_client().send_command("SUNION", *keys)

def SRandMember(key, count=None):
    args = ["SRANDMEMBER", key]
    if count is not None: args.append(count)
    return get_client().send_command(*args)