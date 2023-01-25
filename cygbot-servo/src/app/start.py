import machine
import network
import gc
import time
import select
import sys

from .lib import servo
from . import colorwheel

servo_pin = machine.Pin(10)
my_servo = servo.Servo(servo_pin, min_us=500, max_us=2500)


def kbhit():
    spoll = select.poll()        # Set up an input polling object.
    spoll.register(sys.stdin, select.POLLIN)    # Register polling object.

    kbch = sys.stdin.read(1) if spoll.poll(0) else None

    spoll.unregister(sys.stdin)
    return kbch

led1 = colorwheel.ColorWheel(machine.Pin(38, machine.Pin.OUT))

degrees = 0
degrees_direction = True
last_wrote = time.ticks_ms()
stepper = 5
char_buff = ""
disable_rotate = True

while True:
    led1.update()
    now = time.ticks_ms()
    if now - last_wrote > 1000:
        last_wrote = now
        print(f"# {degrees}°")
        my_servo.write_angle(degrees)
        if not disable_rotate:
            if degrees_direction:
                degrees += stepper
            else:
                degrees -= stepper
    if degrees > 180:
        degrees_direction = False
        degrees = 180 - stepper
    elif degrees < 0:
        degrees_direction = True
        degrees = 0 + stepper

    new_char = kbhit()
    if new_char is not None:
        char_buff += new_char
        if "\n" in char_buff:
            inp = char_buff.split("\n", 1)
            cmd = inp[0]
            if cmd.startswith("! "):
                cmd = cmd.strip()
                cmd = cmd[2:]
                val = int(cmd)
                if val < 0:
                    disable_rotate = False
                    degrees = 0
                else:
                    disable_rotate = True
                    degrees = val
            if len(inp) == 2:
                char_buff = inp[1]
