import socket
import io

"""
GoRedis Python Client

This module provides a simple client to interact with a Go-Redis server
using the RESP protocol. It includes basic commands for connecting,
authenticating, and performing common Redis operations.
"""
class GoRedisClient:
    """
    Internal class to handle RESP protocol and socket communication.
    
    This class manages the TCP connection to the Go-Redis server and
    implements the Redis Serialization Protocol (RESP) for sending
    commands and parsing responses.
    """
    def __init__(self, host, port):
        """
        Initialize the socket connection.
        
        Args:
            host (str): The server hostname or IP address.
            port (int): The port number the server is listening on.
        """
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.sock.connect((host, port))
        self.file = self.sock.makefile('rb')

    def send_command(self, *args):
        """
        Send a command and its arguments to the server using RESP.
        
        Returns the parsed response from the server.
        """
        # RESP requires length in BYTES, not characters.
        cmd = f"*{len(args)}\r\n".encode('utf-8')
        for arg in args:
            b_arg = str(arg).encode('utf-8')
            cmd += f"${len(b_arg)}\r\n".encode('utf-8')
            cmd += b_arg + b"\r\n"

        self.sock.sendall(cmd)
        return self._read_response()

    def _read_response(self):
        """
        Read and parse a RESP response from the server.
        
        Handles Simple Strings, Errors, Integers, Bulk Strings, and Arrays.
        """
        line = self.file.readline()
        if not line:
            raise EOFError("Connection closed by server")

        line = line.rstrip(b'\r\n')
        prefix = line[0:1]
        payload = line[1:]

        if prefix == b'+': return payload.decode('utf-8')
        if prefix == b'-': raise Exception(f"Server Error: {payload.decode('utf-8')}")
        if prefix == b':': return int(payload)
        if prefix == b'$':
            length = int(payload)
            if length == -1: return None
            data = self.file.read(length)
            self.file.read(2)
            return data.decode('utf-8')
        if prefix == b'*':
            count = int(payload)
            if count == -1: return None
            return [self._read_response() for _ in range(count)]
        raise Exception(f"Unknown RESP type received: {prefix}")

    def close(self):
        """Close the underlying socket connection."""
        self.sock.close()

"""
Global client instance.
"""
_global_client = None

"""
Connect to the Go-Redis server.
"""
def get_client():
    """
    Retrieve the global client instance.
    
    Raises RuntimeError if Connect() has not been called.
    """
    global _global_client
    if _global_client is None:
        raise RuntimeError("Client not connected. Call Connect() first.")
    return _global_client