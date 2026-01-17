import json
from collections import defaultdict

def split_json_by_category(input_file):
    # 1. Load the original JSON data
    with open(input_file, 'r') as f:
        data = json.load(f)

    # 2. Group records by their 'category'
    categorized_data = defaultdict(dict)
    
    for command, details in data.items():
        category = details.get("category", "uncategorized")
        # Add the command and its details to the respective category dictionary
        categorized_data[category][command] = details

    totalRecord = 0
    # 3. Write each category to its own JSON file
    for category, records in categorized_data.items():
        filename = f"{category}.json"
        with open(filename, 'w') as out_file:
            json.dump(records, out_file, indent=4)
        print(f"Created: {filename:<20}:{len(records):>3}")
        totalRecord += len(records)
    print(f"[INFO] Total records: {totalRecord}")


# Usage:
# Save your input data to 'commands.json' and run:
# split_json_by_category('commands.json')
split_json_by_category('commands.json')