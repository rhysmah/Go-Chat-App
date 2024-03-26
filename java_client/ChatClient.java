import java.io.*;
import java.net.*;

public class ChatClient {
    public static void main(String[] args) throws IOException {
        var host = "localhost";
        var port = 6666;

        var socket = new Socket(host, port);
        var input = new BufferedReader(new InputStreamReader(socket.getInputStream()));
        var output = new PrintWriter(socket.getOutputStream(), true);
        var stdin = new BufferedReader(new InputStreamReader(System.in));

        // Thread for reading server responses and printing them to the console
        new Thread(() -> {
            String response;
            try {
                while ((response = input.readLine()) != null) {
                    System.out.println(response);
                    System.out.print("> "); // Prompt after displaying the message
                }
            } catch (IOException e) {
                // Stream closed
            }
        }).start();

        // Main thread for accepting and sending user input to the server
        String userInput;
        while (true) {
            System.out.print("> ");
            if ((userInput = stdin.readLine()) == null || userInput.equalsIgnoreCase("/quit")) {
                break; // Exit loop if user types "/quit"
            }
            output.println(userInput);
        }
        socket.close(); // Close the socket when done
    }
}
