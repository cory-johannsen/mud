import os
import json
from mcp.server.fastmcp import FastMCP
from thefuzz import process

# Update this path to your local data
DATA_PATH = r"/home/cjohannsen/src/pf2ools-data/core"

mcp = FastMCP("Pathfinder2E-Global")

def build_global_index():
    """Maps every rule name to its file path across all categories."""
    global_map = {}
    # Iterate through all subfolders (spell, feat, creature, etc.)
    for root, _, files in os.walk(DATA_PATH):
        for file in files:
            if file.endswith(".json"):
                # Use the filename as the key (e.g., 'fireball.json' -> 'fireball')
                name_key = file.replace(".json", "").replace(";", ":").replace("_", " ")
                global_map[name_key] = os.path.join(root, file)
    return global_map

# Load index once on startup
INDEX = build_global_index()

@mcp.tool()
def pf2e_search(query: str) -> str:
    """
    Search all Pathfinder 2E rules (Spells, Feats, Creatures, Items, Conditions).
    Use this for any general Pathfinder rules question.
    """
    # Find the top 3 closest matches to the user's query
    matches = process.extract(query, INDEX.keys(), limit=3)

    # Filter for high-confidence matches (score > 60)
    good_matches = [m for m in matches if m[1] > 60]

    if not good_matches:
        return f"No rules found matching '{query}'."

    results = []
    for match_name, score, _ in good_matches:
        file_path = INDEX[match_name]
        with open(file_path, 'r', encoding='utf-8') as f:
            data = json.load(f)
            # Add a header to show which category this came from
            category = os.path.basename(os.path.dirname(file_path))
            results.append(f"--- MATCH: {match_name.title()} ({category}) --- \n{json.dumps(data, indent=2)}")

    return "\n\n".join(results)

if __name__ == "__main__":
    mcp.run()