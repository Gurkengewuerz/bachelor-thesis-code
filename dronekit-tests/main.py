# pylint: skip-file
import time
import dronekit
import math
import threading
import socket

CONST_LEFT = "LEFT"
CONST_RIGHT = "RIGHT"
CONST_TOP = "TOP"
CONST_DOWN = "DOWN"
CONST_FRONT = "FRONT"
CONST_BATTERY = "BATTERY"
CONST_LANDED = "LANDED"

CONST_BATTERY_CRITICAL = 4 * 3.4

CONST_TARGET_START_ALT_MILLIMETERS = 750
CONST_TARGET_LAND_ALT_MILLIMETERS = 80
CONST_MAX_TAKEOFF_THRUST = 0.55
CONST_NEEDED_TAKEOFF_DIFF_MILLIMETERS = 5

sensor_data = {
  CONST_RIGHT: -1,
  CONST_LEFT: -1,
  CONST_DOWN: -1,
  CONST_TOP: -1,
  CONST_FRONT: -1,
  CONST_BATTERY: -1,
  CONST_LANDED: False,
}

class SensorPull(threading.Thread):
  die = False
  sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
  addr = ("192.168.2.2", 5566)

  def __init__(self):
    threading.Thread.__init__(self)
    self.connect()

  def connect(self):
    self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    print( "connection lost... reconnecting" )  
    while True:  
        try:  
            self.sock.connect(self.addr)  
            print( "re-connection successful" )  
            break
        except socket.error:  
            time.sleep(0.5)  

  def run (self):
    while not self.die and not sensor_data[CONST_LANDED]:
      try: 
        recv = self.sock.recv(1024) 
        data = recv.decode("utf-8")
        data = data.strip()

        evo, sens1, sens2, sens3, sens4 = data.split(",")

        if evo.isnumeric():
          sensor_data[CONST_FRONT] = int(evo)
        
        if sens1.isnumeric():
          sensor_data[CONST_LEFT] = int(sens1)

        if sens2.isnumeric():
          sensor_data[CONST_TOP] = int(sens2)

        if sens3.isnumeric():
          sensor_data[CONST_DOWN] = int(sens3)

        if sens4.isnumeric():
          sensor_data[CONST_RIGHT] = int(sens4)

        print(data)
      except socket.error: 
        self.connect()

      if self.die:
        try:
          self.sock.close()
        except:
          pass

  def join(self):
    print("Stop SensorPull gracefully")
    self.die = True
    super().join()


class Drone(threading.Thread):
  die = False
  vehicle = dronekit.connect('udpbcast:192.168.2.1:14550', baud=115200)

  def __init__(self):
    threading.Thread.__init__(self)
    
  def run (self):
    print("Testing for correct sensor values")

    for position in [CONST_FRONT, CONST_TOP, CONST_DOWN]:
      print("Testing sensor {}".format(position))
      while sensor_data[position] <= 0:
        print("Waiting for sensor {} to get correct data".format(position))
        if self.die:
          return
        time.sleep(1)

    print("Connecting to drone via MAVlink...")
    self.vehicle.wait_ready('autopilot_version')
    print("Drone connected")

    """
    bat = self.vehicle.battery.voltage
    if bat is None:
      print("No Battery connected")
      return

    if bat <= CONST_BATTERY_CRITICAL:
      print("Battery ({}) is below {}V. This way we never start!".format(bat, CONST_BATTERY_CRITICAL))
      return
    """ 
    print("Arming drone...")
    for i in reversed(range(1, 6)):
      print("{}...".format(i))
      time.sleep(1)
      if self.die:
        return

    self.vehicle.mode = dronekit.VehicleMode("GUIDED_NOGPS")
    self.vehicle.armed = True

    while not self.vehicle.armed:
      print("Waiting for arming...")
      if self.die:
        return
      self.vehicle.armed = True
      time.sleep(1)

    print("***Drone is armed***")
    print("Preparing take off")
    for i in reversed(range(1, 4)):
      print("{}...".format(i))
      time.sleep(1)
      if self.die:
        return
    print("TAKE OFF!")
    print("Takeoff to {}mm".format(CONST_TARGET_START_ALT_MILLIMETERS))

    thrust = 0.3
    last_altitude = None
    reached_max_thrust = False
    while True:
      current_altitude = sensor_data[CONST_DOWN]
      print("Altitude: {}mm\t\tDesired: {}mm\t\tThrust: {}".format(current_altitude, CONST_TARGET_START_ALT_MILLIMETERS, round(thrust, 2)))
      if self.die:
        return
      if current_altitude >= CONST_TARGET_START_ALT_MILLIMETERS * 0.95: # Trigger just below target alt.
        break
      
      if reached_max_thrust == False and (last_altitude is None or current_altitude - CONST_NEEDED_TAKEOFF_DIFF_MILLIMETERS <= last_altitude):
        thrust += 0.005
        thrust = min(thrust, CONST_MAX_TAKEOFF_THRUST)
        last_altitude = current_altitude
      else:
        print("Reached max thrust")
        reached_max_thrust = True

      self.set_attitude(thrust = thrust)
      time.sleep(0.2)

    print("Reached target altitude")
    print("Hold position")
    self.set_attitude(duration = 10)
    if self.die:
      return
    #print("Testing YAW")
    #self.set_attitude(yaw_rate = 30, thrust = 0.5, duration = 3)
    #if self.die:
    #  return
    self.force_stop()

  def force_stop(self):
    if self.vehicle.armed:
      print("Starting landing...")

      thrust = 0.45
      while True:
        current_altitude = sensor_data[CONST_DOWN]
        print("Altitude: {}mm\t\tDesired: {}mm\t\tThrust: {}".format(current_altitude, CONST_TARGET_LAND_ALT_MILLIMETERS, round(thrust, 2)))
        if current_altitude <= CONST_TARGET_LAND_ALT_MILLIMETERS:
          break
        elif current_altitude <= CONST_TARGET_LAND_ALT_MILLIMETERS * 2.5:
          thrust = 0.35
        self.set_attitude(thrust = thrust)
        time.sleep(0.2)
      
      print("Reached landing altitude")
      while self.vehicle.armed:
        print("Disarming Vehicle...")
        self.set_attitude(thrust = 0.1)
        self.vehicle.armed = False
        time.sleep(1)

    print("Close vehicle object")
    try:
      self.vehicle.close()
    except:
      pass
    sensor_data[CONST_LANDED] = True

  def send_attitude_target(self, roll_angle = 0.0, pitch_angle = 0.0, yaw_angle = None, yaw_rate = 0.0, use_yaw_rate = False, thrust = 0.5):
    """
    use_yaw_rate: the yaw can be controlled using yaw_angle OR yaw_rate. When one is used, the other is ignored by Ardupilot.
    thrust: 0 <= thrust <= 1, as a fraction of maximum vertical thrust. Note that as of Copter 3.5, thrust = 0.5 triggers a special case in the code for maintaining current altitude.
    Thrust >  0.5: Ascend
    Thrust == 0.5: Hold the altitude
    Thrust <  0.5: Descend
    """
    if yaw_angle is None:
      # this value may be unused by the vehicle, depending on use_yaw_rate
      yaw_angle = self.vehicle.attitude.yaw

    msg = self.vehicle.message_factory.set_attitude_target_encode(
      0, # time_boot_ms
      1, # Target system
      1, # Target component
      0b00000000 if use_yaw_rate else 0b00000100,
      self.to_quaternion(roll_angle, pitch_angle, yaw_angle), # Quaternion
      0, # Body roll rate in radian
      0, # Body pitch rate in radian
      math.radians(yaw_rate), # Body yaw rate in radian/second
      thrust  # Thrust
    )
    self.vehicle.send_mavlink(msg)

  def set_attitude(self, roll_angle = 0.0, pitch_angle = 0.0, yaw_angle = None, yaw_rate = 0.0, use_yaw_rate = False, thrust = 0.5, duration = 0):
    self.send_attitude_target(roll_angle, pitch_angle, yaw_angle, yaw_rate, False, thrust)
    start = time.time()
    while time.time() - start < duration:
      self.send_attitude_target(roll_angle, pitch_angle, yaw_angle, yaw_rate, False, thrust)
      time.sleep(0.1)
      if self.die:
        self.force_stop()
        break
    # Reset attitude, or it will persist for 1s more due to the timeout
    if not self.die:
      self.send_attitude_target(0, 0, 0, 0, True, thrust)

  def to_quaternion(self, roll = 0.0, pitch = 0.0, yaw = 0.0):
    """
    Convert degrees to quaternions
    """
    t0 = math.cos(math.radians(yaw * 0.5))
    t1 = math.sin(math.radians(yaw * 0.5))
    t2 = math.cos(math.radians(roll * 0.5))
    t3 = math.sin(math.radians(roll * 0.5))
    t4 = math.cos(math.radians(pitch * 0.5))
    t5 = math.sin(math.radians(pitch * 0.5))

    w = t0 * t2 * t4 + t1 * t3 * t5
    x = t0 * t3 * t4 - t1 * t2 * t5
    y = t0 * t2 * t5 + t1 * t3 * t4
    z = t1 * t2 * t4 - t0 * t3 * t5

    return [w, x, y, z]


  def join(self):
    print("Stop Drone gracefully")
    self.die = True
    self.force_stop()
    super().join()

if __name__ == "__main__":
  print("*************************************")
  print("* Niklas Schütrumpf                 *")
  print("* Indoor Drone Scanning             *")
  print("* Bachelor Thesis 2022/2023         *")
  print("*************************************")

  sensor = SensorPull()
  sensor.start()

  drone = Drone()
  drone.start()

  try:
    while True:
      time.sleep(1)
      if sensor_data[CONST_LANDED] == True:
        drone.join()
        sensor.join()
  except KeyboardInterrupt:
    drone.join()
    sensor.join()
