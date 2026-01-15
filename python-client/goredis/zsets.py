from .client import get_client

def ZAdd(key, *args):
    """Add one or more members to a sorted set, or update its score. args: score, member, ..."""
    return get_client().send_command("ZADD", key, *args)

def ZRem(key, *members):
    """Remove one or more members from a sorted set."""
    return get_client().send_command("ZREM", key, *members)

def ZScore(key, member):
    """Get the score associated with the given member in a sorted set."""
    return get_client().send_command("ZSCORE", key, member)

def ZCard(key):
    """Get the number of members in a sorted set."""
    return get_client().send_command("ZCARD", key)

def ZRange(key, start, stop, with_scores=False):
    """Return a range of members in a sorted set, by index."""
    args = ["ZRANGE", key, start, stop]
    if with_scores: args.append("WITHSCORES")
    return get_client().send_command(*args)

def ZRevRange(key, start, stop, with_scores=False):
    """Return a range of members in a sorted set, by index, with scores ordered high to low."""
    args = ["ZREVRANGE", key, start, stop]
    if with_scores: args.append("WITHSCORES")
    return get_client().send_command(*args)

def ZGet(key, member=None):
    """Get score of a member or all members with scores."""
    args = ["ZGET", key]
    if member: args.append(member)
    return get_client().send_command(*args)