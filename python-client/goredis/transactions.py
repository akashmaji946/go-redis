from .client import get_client

def Multi():
    return get_client().send_command("MULTI")

def Exec():
    return get_client().send_command("EXEC")

def Discard():
    return get_client().send_command("DISCARD")

def Watch(*keys):
    return get_client().send_command("WATCH", *keys)

def Unwatch():
    return get_client().send_command("UNWATCH")