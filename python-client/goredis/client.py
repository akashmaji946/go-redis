import socket
import io

class GoRedisClient:
    """Internal class to handle RESP protocol and socket communication."""
    def __init__(self, host, port):
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self.sock.connect((host, port))
        self.file = self.sock.makefile('rb')

    def send_command(self, *args):
        # RESP requires length in BYTES, not characters.
        cmd = f"*{len(args)}\r\n".encode('utf-8')
        for arg in args:
            b_arg = str(arg).encode('utf-8')
            cmd += f"${len(b_arg)}\r\n".encode('utf-8')
            cmd += b_arg + b"\r\n"

        self.sock.sendall(cmd)
        return self._read_response()

    def _read_response(self):
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
        self.sock.close()

_global_client = None

def get_client():
    global _global_client
    if _global_client is None:
        raise RuntimeError("Client not connected. Call Connect() first.")
    return _global_client