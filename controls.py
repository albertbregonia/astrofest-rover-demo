while True:
    request = input() # this will be fed by webrtc in golang
    with open('output.txt', 'w') as output: # this is in the body of the loop so i can see output realtime after it closes the file
        output.write(request + '\n') # echo