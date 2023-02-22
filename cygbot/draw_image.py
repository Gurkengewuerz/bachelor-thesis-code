import numpy
import colorsys
from PIL import Image

WIDTH = 160
HEIGHT = 60
SIZE_PER_PIXEL = 4
MAX_DISTANCE = 1500

data_rgb = numpy.zeros((HEIGHT* SIZE_PER_PIXEL, WIDTH * SIZE_PER_PIXEL, 3), dtype=numpy.uint8)
data_gray = numpy.zeros((HEIGHT* SIZE_PER_PIXEL, WIDTH * SIZE_PER_PIXEL), dtype=numpy.uint8)

with open("sensor.dat") as f:
    current_y = 0
    for line in f:
        row_data_str = line.strip()
        row_data = row_data_str.split(" ")
        current_x = 0
        for col in row_data:
            val = float(col)
            for pixel_x in range(SIZE_PER_PIXEL):
                for pixel_y in range(SIZE_PER_PIXEL):
                    hue = colorsys.hsv_to_rgb(val/MAX_DISTANCE, 1, 1)
                    data_rgb[pixel_y + (current_y  * SIZE_PER_PIXEL), pixel_x + (current_x * SIZE_PER_PIXEL)] = [hue[0] * 255, hue[1] * 255, hue[2] * 255]
                    data_gray[pixel_y + (current_y  * SIZE_PER_PIXEL), pixel_x + (current_x * SIZE_PER_PIXEL)] = int((1 - min(val / MAX_DISTANCE, 1)) * 255.9999)
            current_x += 1
        current_y += 1

image_rgb = Image.fromarray(data_rgb)
image_gray = Image.fromarray(data_gray)

image_rgb.save('sensor_rgb.png')
image_gray.save('sensor_gray.png')
