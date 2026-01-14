from .client import get_client

def Publish(channel, message):
    return get_client().send_command("PUBLISH", channel, message)

def Subscribe(*channels):
    return get_client().send_command("SUBSCRIBE", *channels)

def Unsubscribe(*channels):
    return get_client().send_command("UNSUBSCRIBE", *channels)

def PSubscribe(*patterns):
    return get_client().send_command("PSUBSCRIBE", *patterns)

def PUnsubscribe(*patterns):
    return get_client().send_command("PUNSUBSCRIBE", *patterns)