import os
import ssl
import http.server
import socketserver

class CustomHTTPRequestHandler(http.server.SimpleHTTPRequestHandler):
        def end_headers(self):
            # Set custom headers here
            # self.send_header("Access-Control-Allow-Origin", "*")
            # self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            # self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
            super().end_headers()

def run_https_server(directory="../public", port=8443):
    """Run an HTTPS server serving files from the specified directory"""
    # Change to the specified directory
    os.makedirs(directory, exist_ok=True)
    
    # Create handler for serving content
    handler = CustomHTTPRequestHandler
    
    # Set up SSL context
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(certfile="_wildcard.enigma.local.pem", keyfile="_wildcard.enigma.local-key.pem")
    
    # Create server
    server_address = ('', port)
    httpd = socketserver.TCPServer(server_address, handler)
    httpd.socket = context.wrap_socket(httpd.socket, server_side=True)
    # Create custom handler with response headers
    
    print(f"Starting HTTPS server at https://s.enigma.local:{port}")
    print(f"Serving content from directory: {os.path.abspath(directory)}")
    
    # Change to the directory to serve files from
    os.chdir(directory)
    
    # Start the server
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\nServer stopped.")
    finally:
        httpd.server_close()

if __name__ == "__main__":
    run_https_server()
