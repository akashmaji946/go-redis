from .client import get_client

def SAdd(key, *members):
    """Add one or more members to a set."""
    return get_client().send_command("SADD", key, *members)

def SRem(key, *members):
    """Remove one or more members from a set."""
    return get_client().send_command("SREM", key, *members)

def SMembers(key):
    """Get all the members in a set."""
    return get_client().send_command("SMEMBERS", key)

def SIsMember(key, member):
    """Determine if a given value is a member of a set."""
    return get_client().send_command("SISMEMBER", key, member)

def SCard(key):
    """Get the number of members in a set."""
    return get_client().send_command("SCARD", key)

def SDiff(*keys):
    """Subtract multiple sets."""
    return get_client().send_command("SDIFF", *keys)

def SInter(*keys):
    """Intersect multiple sets."""
    return get_client().send_command("SINTER", *keys)

def SUnion(*keys):
    """Add multiple sets."""
    return get_client().send_command("SUNION", *keys)

def SRandMember(key, count=None):
    """Get one or multiple random members from a set."""
    args = ["SRANDMEMBER", key]
    if count is not None: args.append(count)
    return get_client().send_command(*args)