---
project: Cortex
component: Training
phase: Build
date_created: 2026-02-02T15:32:19
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.837445
---

# CortexBrain Model Fine-Tuning — Phase 1 Complete

## Training Dataset Summary

| Metric | Value |
|--------|-------|
| **Source files** | 429 Go files |
| **Source lines** | 88,915 |
| **Functions extracted** | 3,520 |
| **Training pairs** | 3,738 |
| **Total training text** | 3.2M chars (~803K tokens) |
| **Commits** | 275 |
| **Interfaces** | 82 |
| **Brain lobes** | 33 |
| **Packages** | 95+ |

## Training Pair Types

| Type | Count | Description |
|------|-------|-------------|
| `implement_function` | 3,481 | Function signature → implementation |
| `complete_file` | 226 | First half of file → second half |
| `write_module` | 26 | Module path → full source |
| `create_lobe` | 5 | Lobe pattern examples |

## Files Generated

- `/tmp/cortex-training-data/training_pairs.jsonl` (3.5M) — Standard JSONL format
- `/tmp/cortex-training-data/alpaca_format.json` (3.5M) — Alpaca/LoRA format
- `/tmp/cortex-training-data/all_source.go` (2.4M) — Concatenated source
- Copies on Pink at `/tmp/`

## Recommended Base Models for Fine-Tuning

| Model | Params | VRAM (QLoRA) | Speed | Notes |
|-------|--------|-------------|-------|-------|
| **DeepSeek-Coder-V2-Lite** | 2.4B | ~4GB | Fast | Smallest, quick experiments |
| **CodeLlama-7B** | 7B | ~6GB | Good | Well-tested for fine-tuning |
| **DeepSeek-Coder-6.7B** | 6.7B | ~6GB | Good | Strong coding baseline |
| **CodeLlama-13B** | 13B | ~10GB | Slower | Better quality, fits 3090 |

## Phase 2: Fine-Tuning (Next Steps)

1. Choose base model (recommend DeepSeek-Coder-6.7B)
2. Set up QLoRA training on Pink RTX 3090 (or Colab A100)
3. Train for 3-5 epochs (~2-4 hours on 3090)
4. Evaluate on held-out CortexBrain functions
5. Convert to GGUF → deploy in Ollama
6. Replace go-coder with cortex-coder

## Training Command (Draft)

```bash
# On Pink (RTX 3090) or Colab
pip install unsloth peft transformers datasets

python train.py \
  --base_model deepseek-ai/deepseek-coder-6.7b-instruct \
  --data_path /tmp/cortex-training-data/alpaca_format.json \
  --output_dir ./cortex-coder-lora \
  --num_epochs 3 \
  --batch_size 4 \
  --learning_rate 2e-4 \
  --lora_r 16 \
  --lora_alpha 32 \
  --max_seq_length 2048
```
