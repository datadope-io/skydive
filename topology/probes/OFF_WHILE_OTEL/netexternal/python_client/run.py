#! /usr/bin/env python
# -*- coding: utf-8 -*-
# vim:fenc=utf-8
#
# Copyright © 2021 Adrián López Tejedor <adrianlzt@gmail.com>
#
# Distributed under terms of the GNU GPLv3 license.

"""

"""

import sys
import json
from sgqlc.endpoint.http import HTTPEndpoint
from operations import Operations

endpoint = HTTPEndpoint("http://10.20.20.174:4001/query")

print("add switch")
op = Operations.mutation.add_switch_vlan
data = endpoint(op)
json.dump(data, sys.stdout, sort_keys=True, indent=2, default=str)

print("add router")
op = Operations.mutation.add_router_ips
data = endpoint(op)
json.dump(data, sys.stdout, sort_keys=True, indent=2, default=str)


