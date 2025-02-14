import json
import pandas as pd
import matplotlib.pyplot as plt
from clickhouse_driver import Client
from decimal import Decimal
from datetime import datetime

def main():
    client = Client(host='148.251.15.40',user='default', password='clickhouse', database='default')
    query_result = '''SELECT *
FROM default.transactions_details
ARRAY JOIN accounts AS acc
WHERE acc.pubkey = '8pzBGC9KkyssMFuckrcZTN52rhMng5ikpqkmQNoKn45V'
  AND acc.signer = true;'''

    data = client.query_dataframe(query_result)

    print(data)

    

if __name__ == "__main__":
    main()
