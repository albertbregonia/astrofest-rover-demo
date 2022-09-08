# AstroFest 2022 Rover Demo
A simple system in which clients send messages to a web server running on a Raspberry Pi 4B and those messages control GPIO pins connected to a motor controller.

Upon executing `make run`, the resulting executable creates a secure web server that will serve a frontend as the controller for the rover. The frontend contains JavaScript code that will establish a WebSocket and a WebRTC connection with the server. Utilizing the WebSocket to establish the WebRTC connection, the server will then stream the video feed from a Pi-Cam to the frontend and listen on a WebRTC data channel for commands to pass to a piped `controls.py` subprocess. This subprocess controls GPIO pins and merely has an infinite loop waiting for input. HTML elements on the frontend will represent the commands to pass.

If at any point the Python subprocess ends, the parent process (the server) will terminate as the system relies on their coexistence.

Currently, this repository is more of a template than an archive as `controls.py` currently only echos the received text to a file and does not utilize the `RPi.GPIO` package in Python.
