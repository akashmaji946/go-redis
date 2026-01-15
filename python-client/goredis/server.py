from .client import GoRedisClient, get_client
from . import client as client_module

def Connect(host='127.0.0.1', port=7379):
    """Initializes the global connection to the go-redis server."""
    c = GoRedisClient(host, port)
    client_module._global_client = c
    return "Connected"

def Close():
    """Closes the connection."""
    c = get_client()
    c.close()
    client_module._global_client = None

def Auth(username, password=None):
    """Authenticate user to the server."""
    if password is None:
        return get_client().send_command("AUTH", username)
    return get_client().send_command("AUTH", username, password)

def Ping(message=None):
    """Ping the server."""
    args = ["PING"]
    if message: args.append(message)
    return get_client().send_command(*args)

def Select(index):
    """Change the selected database for the current connection."""
    return get_client().send_command("SELECT", index)

def Sel(index):
    """Alias for Select."""
    return Select(index)

def Info(key=None):
    """Get server information and statistics or per-key metadata."""
    args = ["INFO"]
    if key: args.append(key)
    return get_client().send_command(*args)

def Monitor():
    """Listen for all requests received by the server in real time."""
    return get_client().send_command("MONITOR")

def DbSize():
    """Return the number of keys in the database."""
    return get_client().send_command("DBSIZE")

def FlushDb():
    """Remove all keys from the database."""
    return get_client().send_command("FLUSHDB")

def DropDb():
    """Alias for FlushDb."""
    return FlushDb()

def Size(db_index=None):
    """Get the number of databases or size of a specific database."""
    args = ["SIZE"]
    if db_index is not None: args.append(db_index)
    return get_client().send_command(*args)

def UserAdd(admin_flag, user, password):
    """Create a new user (Admin only)."""
    return get_client().send_command("USERADD", admin_flag, user, password)

def Passwd(user, password):
    """Change a user's password."""
    return get_client().send_command("PASSWD", user, password)

def Users(username=None):
    """List all usernames or show specific user details."""
    args = ["USERS"]
    if username: args.append(username)
    return get_client().send_command(*args)

def WhoAmI():
    """Display details of the currently authenticated user."""
    return get_client().send_command("WHOAMI")

def Save():
    """Synchronously save the database to disk."""
    return get_client().send_command("SAVE")

def BgSave():
    """Asynchronously save the database to disk."""
    return get_client().send_command("BGSAVE")

def BgRewriteAof():
    """Asynchronously rewrite the Append-Only File."""
    return get_client().send_command("BGREWRITEAOF")

def Command():
    """Get help about Redis commands."""
    return get_client().send_command("COMMAND")

def Commands(pattern=None):
    """List available commands or get help for a specific command."""
    args = ["COMMANDS"]
    if pattern: args.append(pattern)
    return get_client().send_command(*args)