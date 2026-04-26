import os
import time
import psycopg2
import logging

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

def get_connection():
    db_url = os.environ.get("DATABASE_URL", "postgresql://user:password@postgres:5432/sampleapp")
    return psycopg2.connect(db_url)

def check_stuck_jobs():
    try:
        conn = get_connection()
        cur = conn.cursor()
        
        # Find jobs stuck in PROCESSING for more than 30 seconds
        cur.execute("""
            SELECT id, name, updated_at 
            FROM jobs 
            WHERE status = 'PROCESSING' 
            AND updated_at < NOW() - INTERVAL '30 seconds'
        """)
        
        stuck_jobs = cur.fetchall()
        if stuck_jobs:
            for job in stuck_jobs:
                logging.warning(f"🚨 VERIFIER ALERT: Job {job[0]} '{job[1]}' is stuck! Last updated: {job[2]}")
        else:
            logging.info("Verifier check passed. No stuck jobs found.")
            
        cur.close()
        conn.close()
    except Exception as e:
        logging.error(f"Error checking jobs: {e}")

if __name__ == "__main__":
    logging.info("Verifier service starting up...")
    # Wait for DB to be ready
    time.sleep(5)
    
    while True:
        check_stuck_jobs()
        time.sleep(10)
