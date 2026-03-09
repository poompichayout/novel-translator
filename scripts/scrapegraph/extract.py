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
            "model": "google_genai/gemini-2.5-flash", 
            "max_tokens": 8192
        },
        "verbose": False,
        "headless": True
    }
    
    prompt = """
    Analyze the provided webpage URL and extract its core structured data. 
    1. If this is a novel index page or table of contents, extract the 'title' of the novel and a list of 'chapters', where each chapter has a 'title' and a 'url'.
    2. If this is a single chapter page, extract the 'title' of the chapter and the raw text 'content' of the chapter narrative.
    
    Return the result strictly as a flat JSON object following this structure, do not nest it under a 'content' key:
    {
       "title": "String",
       "chapters": [{"title": "String", "url": "String"}],
       "content": "String"
    }
    Leave fields empty or null if they are not applicable.
    """

    import io
    from contextlib import redirect_stdout, redirect_stderr
    
    try:
        smart_scraper_graph = SmartScraperGraph(
            prompt=prompt,
            source=url,
            config=graph_config
        )
        
        # Save original stdout, point stdout to stderr so scrapegraph prints there
        original_stdout = sys.stdout
        sys.stdout = sys.stderr
        
        result = smart_scraper_graph.run()
        print(f"[DEBUG] ScrapeGraph result: {result}", file=sys.stderr)
        
        sys.stdout = original_stdout
            
        # Sometimes ScrapeGraph nests the result inside a 'content' key, flatten it if needed
        if isinstance(result, dict) and "content" in result and isinstance(result["content"], dict):
            new_result = result["content"]
            # Preserve other keys if they exist outside of content
            for k, v in result.items():
                if k != "content":
                    new_result[k] = v
            result = new_result
            
        print(json.dumps(result))
        
    except Exception as e:
        sys.stdout = original_stdout
        print(json.dumps({"error": str(e)}))
        sys.exit(1)

if __name__ == "__main__":
    main()
