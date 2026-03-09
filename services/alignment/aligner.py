import os
import re
import json
import psycopg2
import psycopg2.extras
import numpy as np
from google import genai
import nltk
from pythainlp.tokenize import sent_tokenize as th_sent_tokenize
from sklearn.metrics.pairwise import cosine_similarity

# Initialize Models
try:
    gemini_api_key = os.environ.get("GEMINI_API_KEY")
    if not gemini_api_key:
        # fallback to grep from config.yaml
        with open("./config.yaml", "r") as f:
            for line in f:
                if "api_key" in line and "TODO" not in line:
                    gemini_api_key = line.split('"')[1]
                    break
except Exception:
    pass

print("Initializing Gemini Client...")
client = genai.Client(api_key=gemini_api_key)
print("Client loaded.")

def clean_text(text):
    if not text:
        return ""
    # Remove basic HTML tags if any sneaked in
    text = re.sub(r'<[^>]+>', '', text)
    # Replace multiple newlines with a single newline
    text = re.sub(r'\n+', '\n', text)
    return text.strip()

def tokenize_sentences_en(text):
    sentences = []
    # Split by paragraph first to avoid NLTK getting confused across hard breaks
    paragraphs = text.split('\n')
    for p in paragraphs:
        p = p.strip()
        if p:
            sentences.extend(nltk.sent_tokenize(p))
    return sentences

def tokenize_sentences_th(text):
    sentences = []
    paragraphs = text.split('\n')
    for p in paragraphs:
        p = p.strip()
        if p:
            # PyThaiNLP sent_tokenize uses deepcut or crf, by default "crfcut"
            sentences.extend(th_sent_tokenize(p, engine="crfcut"))
    # Filter out empty or punctuation-only sentences
    sentences = [s.strip() for s in sentences if len(s.strip()) > 1]
    return sentences

def align_sentences(en_sents, th_sents, similarity_threshold=0.5):
    if not en_sents or not th_sents:
        return []
    
    print(f"Vectorizing {len(en_sents)} EN and {len(th_sents)} TH sentences using Gemini...")
    
    # Batch embeddings
    en_response = client.models.embed_content(
        model="text-embedding-004",
        contents=en_sents
    )
    en_embeddings = np.array([emb.values for emb in en_response.embeddings])
    
    th_response = client.models.embed_content(
        model="text-embedding-004",
        contents=th_sents
    )
    th_embeddings = np.array([emb.values for emb in th_response.embeddings])
    
    sim_matrix = cosine_similarity(en_embeddings, th_embeddings)
    
    aligned_pairs = []
    current_th_idx = 0
    
    for i, en_sent in enumerate(en_sents):
        # We look around the current Thai index to find the best match (sliding window)
        # This prevents jumping backward in time and helps chronological alignment.
        window_start = max(0, current_th_idx - 1)
        window_end = min(len(th_sents), current_th_idx + 5)
        
        if window_start >= len(th_sents):
            break
            
        window_sims = sim_matrix[i, window_start:window_end]
        if len(window_sims) == 0:
            continue
            
        best_local_idx = np.argmax(window_sims)
        best_score = window_sims[best_local_idx]
        global_th_idx = window_start + best_local_idx
        
        if best_score > similarity_threshold:
            aligned_pairs.append({
                "en": en_sent,
                "th": th_sents[global_th_idx],
                "score": float(best_score),
                "en_idx": i,
                "th_idx": global_th_idx
            })
            current_th_idx = global_th_idx # Update chronological anchor
            
    return aligned_pairs

def main():
    db_url = os.getenv("DB_URL", "postgres://translator:password123@localhost:5432/novel_translator")
    print(f"Connecting to DB at {db_url}")
    
    conn = psycopg2.connect(db_url)
    cursor = conn.cursor(cursor_factory=psycopg2.extras.DictCursor)
    
    # 1. Fetch pair
    query = """
        SELECT 
            c1.id as en_chapter_id, c1.raw_content as en_content, c1.chapter_number,
            c2.id as th_chapter_id, c2.raw_content as th_content
        FROM novel_mappings nm
        JOIN chapters c1 ON c1.novel_id = nm.source_novel_id
        JOIN chapters c2 ON c2.novel_id = nm.target_novel_id AND c1.chapter_number = c2.chapter_number
        WHERE c1.raw_content IS NOT NULL AND c2.raw_content IS NOT NULL
          AND NOT EXISTS (
              SELECT 1 FROM translation_pairs tp WHERE tp.chapter_id = c1.id
          )
        LIMIT 1;
    """
    cursor.execute(query)
    row = cursor.fetchone()
    
    if not row:
        print("No unaligned chapter pairs found.")
        return
        
    en_chapter_id = row['en_chapter_id']
    th_chapter_id = row['th_chapter_id']
    chapter_num = row['chapter_number']
    print(f"Found Chapter {chapter_num} to align (EN ID: {en_chapter_id}, TH ID: {th_chapter_id})")
    
    # 2. Clean & Tokenize
    en_text = clean_text(row['en_content'])
    th_text = clean_text(row['th_content'])
    
    en_sents = tokenize_sentences_en(en_text)
    th_sents = tokenize_sentences_th(th_text)
    
    print(f"Tokenized: {len(en_sents)} EN sentences, {len(th_sents)} TH sentences")
    
    # 3. Align
    pairs = align_sentences(en_sents, th_sents, similarity_threshold=0.6)
    print(f"Successfully aligned {len(pairs)} pairs!")
    
    # 4. Save to DB and Export
    train_file = "train.jsonl"
    with open(train_file, "a", encoding="utf-8") as f:
        for pair in pairs:
            # Insert into DB
            insert_query = """
                INSERT INTO translation_pairs 
                (chapter_id, sentence_en, sentence_th, similarity_score, is_validated)
                VALUES (%s, %s, %s, %s, %s)
            """
            cursor.execute(insert_query, (
                en_chapter_id, pair['en'], pair['th'], pair['score'], False
            ))
            
            # Write to JSONL
            json_record = {
                "instruction": "Translate the following English sentence to Thai:",
                "input": pair["en"],
                "output": pair["th"]
            }
            f.write(json.dumps(json_record, ensure_ascii=False) + "\n")
            
    conn.commit()
    cursor.close()
    conn.close()
    print(f"Alignment completed for chapter {chapter_num}. Saved to DB and {train_file}.")

if __name__ == "__main__":
    main()
