from .client import get_client

def Multi():
    """Mark the start of a transaction block."""
    return get_client().send_command("MULTI")

def Exec():
    """Execute all commands queued in a transaction."""
    return get_client().send_command("EXEC")

def Discard():
    """Discard all commands issued after MULTI."""
    return get_client().send_command("DISCARD")

def Watch(*keys):
    """Watch the given keys to determine execution of the MULTI/EXEC block."""
    return get_client().send_command("WATCH", *keys)

def Unwatch():
    """Forget about all watched keys."""
    return get_client().send_command("UNWATCH")