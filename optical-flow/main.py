import cv2
import numpy as np

# https://stackoverflow.com/a/44659589/5605489
def image_resize(image, width = None, height = None, inter = cv2.INTER_AREA):
    # initialize the dimensions of the image to be resized and
    # grab the image size
    dim = None
    (h, w) = image.shape[:2]

    # if both the width and height are None, then return the
    # original image
    if width is None and height is None:
        return image

    # check to see if the width is None
    if width is None:
        # calculate the ratio of the height and construct the
        # dimensions
        r = height / float(h)
        dim = (int(w * r), height)

    # otherwise, the height is None
    else:
        # calculate the ratio of the width and construct the
        # dimensions
        r = width / float(w)
        dim = (width, int(h * r))

    # resize the image
    resized = cv2.resize(image, dim, interpolation = inter)

    # return the resized image
    return resized

img_one = cv2.imread('optical-flow/testdata/40.png')
img_one = image_resize(img_one, height = 800)
img_one_gray = cv2.cvtColor(img_one, cv2.COLOR_BGR2GRAY)

img_two = cv2.imread('optical-flow/testdata/42.png')
img_two = image_resize(img_two, height = 800)
img_two_gray = cv2.cvtColor(img_two, cv2.COLOR_BGR2GRAY)

img_out = img_two.copy()

flow = cv2.calcOpticalFlowFarneback(img_one_gray, img_two_gray, None, 0.5, 3, 15, 3, 5, 1.2, 0)

rows, cols = img_two_gray.shape

steps = 15
for y in range(0, rows, steps):
    for x in range(0, cols, steps):
        flow_x, flow_y = flow[y, x] * 15
        cv2.line(img_out, (x, y), (round(x + flow_x), round(y + flow_y)), (190, 150, 37), 2) 
        cv2.circle(img_out, (x, y), 1, (255, 0, 0), 1) 


cv2.imshow('frame2', img_out)
cv2.waitKey(0)
cv2.destroyAllWindows()
cv2.imwrite("optical-flow/output.jpg", img_out)