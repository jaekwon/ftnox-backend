#!/usr/bin/python
#
# common-bin
#
# ex: echo test 2>&1 | log ~/test.log
#

import sys
import os
import os.path
import datetime

MAX_FILE_SIZE = 20 * 1024 * 1024
MAX_LOG_ROTATE = 200 # TOO HIGH AND PYTHON STACKOVERFLOWS AROUND 9XX by default.

def rotate2(f, i1, i2):
  to1 = f + "." + str(i1)
  to2 = f + "." + str(i2)
  if os.path.isfile(to2) and i2 < MAX_LOG_ROTATE:
    rotate2(f, i2, i2 + 1)
  os.rename(to1, to2)

def rotate1(f, i):
  to = f + "." + str(i)
  if os.path.isfile(to):
    rotate2(f, i, i + 1)
  os.rename(f, to)

def rotate(f):
  rotate1(f, 1)

def is_rotate(o):
  if not os.path.isfile(o):
    return False
  if os.path.getsize(o) > MAX_FILE_SIZE:
    return True
  d = datetime.datetime.today()
  c = datetime.datetime.fromtimestamp(os.path.getctime(o))
  if d.date() != c.date():
    return True
  return False

o = sys.argv[1]

if is_rotate(o):
  rotate(o)

while True:
  l = sys.stdin.readline()
  if not l:
    break
  if is_rotate(o):
    rotate(o)
  with open(o, "a", 0) as f:
    f.write(l)
