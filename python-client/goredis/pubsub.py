from .client import get_client

def Publish(channel, message):
    """Post a message to a channel."""
    return get_client().send_command("PUBLISH", channel, message)

def Pub(channel, message):
    """Alias for Publish."""
    return Publish(channel, message)

def Subscribe(*channels):
    """Listen for messages published to the given channels."""
    return get_client().send_command("SUBSCRIBE", *channels)

def Sub(*channels):
    """Alias for Subscribe."""
    return Subscribe(*channels)

def Unsubscribe(*channels):
    """Stop listening for messages posted to the given channels."""
    return get_client().send_command("UNSUBSCRIBE", *channels)

def Unsub(*channels):
    """Alias for Unsubscribe."""
    return Unsubscribe(*channels)

def PSubscribe(*patterns):
    """Listen for messages published to channels matching the given patterns."""
    return get_client().send_command("PSUBSCRIBE", *patterns)

def PSub(*patterns):
    """Alias for PSubscribe."""
    return PSubscribe(*patterns)

def PUnsubscribe(*patterns):
    """Stop listening for messages posted to channels matching the given patterns."""
    return get_client().send_command("PUNSUBSCRIBE", *patterns)

def PUnsub(*patterns):
    """Alias for PUnsubscribe."""
    return PUnsubscribe(*patterns)