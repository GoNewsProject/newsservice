CREATE TABLE IF NOT EXISTS news(
  id SERIAL PRIMARY KEY,
  title VARCHAR(500) NOT NULL,
  description TEXT NOT NULL,
  content TEXT NOT NULL,
  author VARCHAR(200) NOT NULL,
  published_at TIMESTAMP NOT NULL,
  source VARCHAR(200) NOT NULL,
  link VARCHAR(1000) NOT NULL UNIQUE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);

-- CREATE INDEX IF NOT EXISTS idx_news_published_at ON news(published_at DESC);
-- CREATE INDEX IF NOT EXISTS idx_news_source ON news(source);
-- CREATE INDEX IF NOT EXISTS idx_news_author ON news(author);
-- CREATE INDEX IF NOT EXISTS idx_news_link ON news(link);