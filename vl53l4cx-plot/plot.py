import glob
import json
import os

import matplotlib.pyplot as plt
from matplotlib.patches import Rectangle
import numpy as np


fig = plt.figure()
ax1 = fig.add_subplot()

xData = []
yData_ref = []
yData_indoor = []
yData_outdoor = []

with open("data.csv", "r") as f:
    for line in f.readlines():
        data = line.strip().split(";")
        xData.append(int(data[0]))
        yData_ref.append(int(data[0]))
        yData_indoor.append(int(data[1]))
        if int(data[2]) != -1:
            yData_outdoor.append(int(data[2]))
        else:
            yData_outdoor.append(np.nan)
        print(f"{data[0]}cm & {data[1]}cm & {data[2]}cm \\\\")

# First 10cm are dead
#ax1.add_patch(Rectangle((0, 0), 10, 10))
line, = ax1.plot(xData, yData_ref, color="green")
line.set_label("Referenz")

line, = ax1.plot(xData, yData_indoor, color="red")
line.set_label("abgedunkelter Innenraum")

line, = ax1.plot(xData, yData_outdoor, color="blue")
line.set_label("Draußen")

ax1.grid(color='grey', linestyle='-', linewidth=0.25, alpha=0.5)
ax1.set_xlabel('Soll-Abstand in $cm$')
ax1.legend()
ax1.set_ylabel('Ist-Abstand in $cm$')
fig.savefig("graph.png")
