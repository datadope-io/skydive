python -m sgqlc.introspection --exclude-deprecated --exclude-description http://10.20.20.174:4001/query schema.json
sgqlc-codegen schema schema.json netexternal.py
cp operations.py operations.py.bkp.$(date +%Y%m%d%H%M)
sgqlc-codegen operation --schema schema.json netexternal operations.py mutation_add_switch.gql mutation_add_router.gql
echo "operations.py necesita algún retoque para funcionar, mirar alguno de los de backup"
