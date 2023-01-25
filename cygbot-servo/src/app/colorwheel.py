import time
import math
import neopixel

class ColorWheel:

    def __init__(self, pin) -> None:
        self._pin = pin
        self._neopixel = neopixel.NeoPixel(self._pin, 1)
        self._last = 0
        self._hue = 0
        self._hue_state = False
    
    def hsv_to_rgb(self, h, s, v):
        i = math.floor(h*6)
        f = h*6 - i
        p = v * (1-s)
        q = v * (1-f*s)
        t = v * (1-(1-f)*s)

        r, g, b = [
            (v, t, p),
            (q, v, p),
            (p, v, t),
            (p, q, v),
            (t, p, v),
            (v, p, q),
        ][int(i%6)]

        return r, g, b
    
    def update(self):
        if time.ticks_ms() - self._last < 100:
            return
        
        r, g, b = self.hsv_to_rgb(self._hue/360., 1, 1)
        self._neopixel[0] = (int(r * 255), int(g * 255), int(b * 255))
        self._neopixel.write()

        if self._hue_state:
            self._hue -= 1
        else:
            self._hue += 1

        if self._hue < 0 or self._hue > 360:
            self._hue_state = not self._hue_state

        self._last = time.ticks_ms()
