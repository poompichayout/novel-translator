import sys
import json
import asyncio
import nest_asyncio
import os
from scrapegraphai.graphs import SmartScraperGraph

nest_asyncio.apply()

def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "URL target is required"}))
        sys.exit(1)
        
    url = sys.argv[1]
    
    # Retrieve GEMINI_API_KEY from environment variables instead of hardcoding
    api_key = os.environ.get("GEMINI_API_KEY")
    if not api_key:
        print(json.dumps({"error": "GEMINI_API_KEY environment variable is not set"}))
        sys.exit(1)

    graph_config = {
        "llm": {
            "api_key": api_key,
            "model": "gemini-2.5-flash", 
        },
        "verbose": False,
        "headless": True
    }
    
    prompt = """
    Analyze the provided webpage URL and extract its core structured data. 
    1. If this is a novel index page or table of contents, extract the 'title' of the novel and a list of 'chapters', where each chapter has a 'title' and a 'url'.
    2. If this is a single chapter page, extract the 'title' of the chapter and the raw text 'content' of the chapter narrative.
    
    Return the result as a JSON object strictly following this structure:
    {
       "title": "String",
       "chapters": [{"title": "String", "url": "String"}],
       "content": "String"
    }
    Leave fields empty or null if they are not applicable (e.g., 'chapters' should be null/empty on a single chapter page).
    """

    try:
        smart_scraper_graph = SmartScraperGraph(
            prompt=prompt,
            source=url,
            config=graph_config
        )
        
        result = smart_scraper_graph.run()
        print(json.dumps(result))
        
    except Exception as e:
        print(json.dumps({"error": str(e)}))
        sys.exit(1)

if __name__ == "__main__":
    main()
