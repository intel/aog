# Embed Service Example

This example demonstrates how to use the AOG Embed API to generate text embeddings.

## 📝 Scenario Description

The Embed service can:
- Convert text into high-dimensional vector representations
- Enable semantic search and similarity calculations
- Support batch text embedding
- Facilitate text clustering and classification

## 🎯 Learning Objectives

Through this example, you will learn:
1. How to call the AOG Embed API
2. How to process embedding vectors
3. How to calculate text similarity
4. How to perform semantic search

## 🔌 API Endpoint

```
POST http://localhost:16688/aog/v0.2/services/embed
```

## 📋 Request Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | No | Model name, e.g., `text-embedding-v3` |
| `input` | array | Yes | Array of texts to generate embeddings for |

### Request Example

```json
{
  "model": "text-embedding-v3",
  "input": [
    "This is the first text",
    "This is the second text"
  ]
}
```

## 📊 Response Format

```json
{
  "data": [
    {
      "embedding": [-0.0695, 0.0306, ...],
      "index": 0,
      "object": "embedding"
    },
    {
      "embedding": [-0.0634, 0.0604, ...],
      "index": 1,
      "object": "embedding"
    }
  ],
  "model": "text-embedding-v3",
  "id": "73591b79-d194-9bca-8bb5-xxxxxxxxxxxx"
}
```

## 🚀 Quick Start

### Prerequisites

1. ✅ AOG service is installed and running
2. ✅ Embed service is installed
3. ✅ Required embedding model is downloaded (e.g., `text-embedding-v3`)

### Steps

1. Ensure AOG service is running
2. Open [embed.ipynb](./embed.ipynb)
3. Execute the code cells in the notebook sequentially

## 💡 Use Cases

### 1. Semantic Search

```python
# Generate embeddings for query and documents
query_embedding = get_embedding("user query")
doc_embeddings = get_embeddings(["Document 1", "Document 2", "Document 3"])

# Calculate similarity to find most relevant documents
similarities = cosine_similarity(query_embedding, doc_embeddings)
```

### 2. Text Clustering

```python
# Generate embeddings for multiple texts
embeddings = get_embeddings(texts)

# Use clustering algorithm (e.g., K-means) to group texts
from sklearn.cluster import KMeans
kmeans = KMeans(n_clusters=3)
clusters = kmeans.fit_predict(embeddings)
```

### 3. Similarity Calculation

```python
# Calculate similarity between two texts
text1_emb = get_embedding("Text 1")
text2_emb = get_embedding("Text 2")
similarity = cosine_similarity(text1_emb, text2_emb)
```

## 🔍 FAQ

**Q: What is the dimension of embedding vectors?**  
A: Depends on the model used, typically between 384 and 1536 dimensions.

**Q: How to choose the right model?**  
A: Choose based on your use case:
- General text: `text-embedding-v3`
- Multilingual: Choose models that support multiple languages
- Domain-specific: Use domain-specialized models

**Q: What's the maximum batch size?**  
A: Recommend no more than 100 texts per request to avoid timeouts.

**Q: How to store and retrieve embedding vectors?**  
A: Use vector databases (e.g., Milvus, Pinecone, Weaviate) for efficient storage and retrieval.

## 📚 Related Resources

- [AOG API Documentation](../../docs/)
- [Back to Main](../README.md)
- [Chat Service Example](../text-generation/)
