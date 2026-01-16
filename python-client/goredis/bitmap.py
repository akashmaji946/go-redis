from .client import get_client

def SetBit(key, offset, value):
    """Set or clear the bit at the specified offset in the string value stored at key."""
    return get_client().send_command("SETBIT", key, offset, value)

def GetBit(key, offset):
    """Return the bit value at the specified offset in the string value stored at key."""
    return get_client().send_command("GETBIT", key, offset)

def BitCount(key, *args):
    """Count the number of set bits (population counting) in a string."""
    return get_client().send_command("BITCOUNT", key, *args)

def BitOp(operation, dest_key, *keys):
    """Perform a bitwise operation between multiple source strings and store the result in the destination key."""
    return get_client().send_command("BITOP", operation, dest_key, *keys)

def BitPos(key, bit, *args):
    """Return the position of the first bit set to 0 or 1 in a string."""
    return get_client().send_command("BITPOS", key, bit, *args)

def BitField(key, *operations):
    """Perform arbitrary bitfield integer operations on strings."""
    return get_client().send_command("BITFIELD", key, *operations)
