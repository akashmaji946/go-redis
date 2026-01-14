from .client import get_client

def ZAdd(key, *args):
    # args should be score, member, score, member...
    return get_client().send_command("ZADD", key, *args)

def ZRem(key, *members):
    return get_client().send_command("ZREM", key, *members)

def ZScore(key, member):
    return get_client().send_command("ZSCORE", key, member)

def ZCard(key):
    return get_client().send_command("ZCARD", key)

def ZRange(key, start, stop, with_scores=False):
    args = ["ZRANGE", key, start, stop]
    if with_scores: args.append("WITHSCORES")
    return get_client().send_command(*args)

def ZRevRange(key, start, stop, with_scores=False):
    args = ["ZREVRANGE", key, start, stop]
    if with_scores: args.append("WITHSCORES")
    return get_client().send_command(*args)

def ZGet(key, member=None):
    args = ["ZGET", key]
    if member: args.append(member)
    return get_client().send_command(*args)