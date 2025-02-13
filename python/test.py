import pandas as pd
import matplotlib.pyplot as plt
from datetime import datetime

# Read and process the JSON data
df = pd.read_json('sample.json')

# Function to extract post_sol_balance for specific account
def get_account_balance(accounts, target_pubkey):
    for account in accounts:
        if account['pubkey'] == target_pubkey:
            return float(account['post_sol_balance'])
    return None

# Function to safely convert timestamp
def safe_timestamp_to_readable(timestamp):
    try:
        # Convert timestamp to milliseconds if it's too large
        if len(str(timestamp)) > 10:
            timestamp = int(timestamp) / 1000
        return datetime.fromtimestamp(timestamp).strftime('%Y-%m-%d %H:%M:%S')
    except (ValueError, OSError, TypeError):
        return str(timestamp)

# Extract balance for the specific account
target_account = '6SzeGe3uXNwFxsjResVMJgPzfZ3v3BbFCLQKUoMibQQR'
df['post_balance'] = df['accounts'].apply(lambda x: get_account_balance(x, target_account))

# Convert block_time to numeric and then to readable format
df['block_time'] = pd.to_numeric(df['block_time'])
df['datetime_str'] = df['block_time'].apply(safe_timestamp_to_readable)

# Print the timestamp values for debugging
print("Original timestamps:")
print(df[['block_time', 'datetime_str']].head())

# Sort by block_time
df = df.sort_values('block_time')

# Create the plot
plt.figure(figsize=(12, 6))
plt.plot(range(len(df)), df['post_balance'], marker='o', linestyle='-', linewidth=2, markersize=8)

# Customize the plot
plt.title(f'SOL Balance Over Time for Account: {target_account[:10]}...', fontsize=12)
plt.xlabel('Time Points', fontsize=10)
plt.ylabel('Post SOL Balance', fontsize=10)
plt.grid(True, linestyle='--', alpha=0.7)

# Set x-ticks to show datetime strings
plt.xticks(range(len(df)), df['datetime_str'], rotation=45, ha='right')

# Adjust layout to prevent label cutoff
plt.tight_layout()

# Display the plot
plt.show()

# Print the data points for verification
print("\nData Points:")
print(df[['datetime_str', 'post_balance']].to_string())