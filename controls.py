import RPi.GPIO as GPIO

relay1_pin = 11
relay2_pin = 13

m1_dir_pin = 16
m2_dir_pin = 31

m1_pwm_pin = 12
m2_pwm_pin = 33

m1_pwm = None
m2_pwm = None

motors_enabled = False

def setup():
    GPIO.setmode(GPIO.BOARD)
    # all pins set to output
    GPIO.setup(relay1_pin, GPIO.OUT)
    GPIO.setup(relay2_pin, GPIO.OUT)
    GPIO.setup(m1_dir_pin, GPIO.OUT)
    GPIO.setup(m2_dir_pin, GPIO.OUT)
    GPIO.setup(m1_pwm_pin, GPIO.OUT)
    GPIO.setup(m2_pwm_pin, GPIO.OUT)

    # sets the pwm pins for the motors to operate at 490Hz
    global m1_pwm
    global m2_pwm
    m1_pwm = GPIO.PWM(m1_pwm_pin, 490)
    m2_pwm = GPIO.PWM(m2_pwm_pin, 490)


def enableMotors():
    GPIO.output(relay1_pin, GPIO.LOW)
    GPIO.output(relay2_pin, GPIO.LOW)

    GPIO.output(m1_dir_pin, GPIO.HIGH)
    GPIO.output(m2_dir_pin, GPIO.HIGH)

    m1_pwm.start(0)
    m2_pwm.start(0)

def disableMotors():
    m1_pwm.stop()
    m2_pwm.stop()

    GPIO.output(relay1_pin, GPIO.LOW)
    GPIO.output(relay2_pin, GPIO.LOW)

def move(m1, m2, max_speed):
    if m1 >= 0:
        GPIO.output(m1_dir_pin, GPIO.HIGH)
    else:
        GPIO.output(m1_dir_pin, GPIO.LOW)

    if m2 >= 0:
        GPIO.output(m2_dir_pin, GPIO.HIGH)
    else:
        GPIO.output(m2_dir_pin, GPIO.LOW)

    m1_pwm.ChangeDutyCycle(abs(m1) * max_speed)
    m2_pwm.ChangeDutyCycle(abs(m2) * max_speed)

# if speed is positive, dir pin is set to LOW

setup()

while True:
    request = input() # this will be fed by webrtc in golang
    [motor_status, m1, m2, speed] = request.split("::")

    if bool(motor_status) and not motors_enabled:
        enableMotors()
        motors_enabled = True
    elif bool(motor_status) and motors_enabled:
        disableMotors()
        motors_enabled = False

    move(int(m1), int(m2), int(speed) / 100)
