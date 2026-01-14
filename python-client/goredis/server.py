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
    if password is None:
        return get_client().send_command("AUTH", username)
    return get_client().send_command("AUTH", username, password)

def Ping(message=None):
    args = ["PING"]
    if message: args.append(message)
    return get_client().send_command(*args)

def Select(index):
    return get_client().send_command("SELECT", index)

def Info(key=None):
    args = ["INFO"]
    if key: args.append(key)
    return get_client().send_command(*args)

def Monitor():
    return get_client().send_command("MONITOR")

def DbSize():
    return get_client().send_command("DBSIZE")

def FlushDb():
    return get_client().send_command("FLUSHDB")

def Size(db_index=None):
    args = ["SIZE"]
    if db_index is not None: args.append(db_index)
    return get_client().send_command(*args)

def UserAdd(admin_flag, user, password):
    return get_client().send_command("USERADD", admin_flag, user, password)

def Passwd(user, password):
    return get_client().send_command("PASSWD", user, password)

def Users(username=None):
    args = ["USERS"]
    if username: args.append(username)
    return get_client().send_command(*args)

def WhoAmI():
    return get_client().send_command("WHOAMI")

def Save():
    return get_client().send_command("SAVE")

def BgSave():
    return get_client().send_command("BGSAVE")

def BgRewriteAof():
    return get_client().send_command("BGREWRITEAOF")

def Command():
    return get_client().send_command("COMMAND")

def Commands(pattern=None):
    args = ["COMMANDS"]
    if pattern: args.append(pattern)
    return get_client().send_command(*args)