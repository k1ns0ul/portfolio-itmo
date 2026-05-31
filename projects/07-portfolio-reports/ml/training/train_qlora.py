from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from pathlib import Path

from config import LLMConfig

log = logging.getLogger(__name__)


@dataclass
class TrainArtifacts:
    adapter_dir: Path
    train_examples: int
    epochs: int
    base_model: str


class QLoRATrainer:
    def __init__(self, cfg: LLMConfig) -> None:
        self.cfg = cfg

    def train(self, dataset_path: str | Path) -> TrainArtifacts:
        import torch
        from datasets import load_dataset
        from peft import LoraConfig, get_peft_model, prepare_model_for_kbit_training
        from transformers import (AutoModelForCausalLM, AutoTokenizer,
                                  BitsAndBytesConfig, DataCollatorForLanguageModeling,
                                  Trainer, TrainingArguments)

        dataset_path = Path(dataset_path)
        if not dataset_path.exists():
            raise FileNotFoundError(f"dataset not found at {dataset_path}")

        bnb = BitsAndBytesConfig(
            load_in_4bit=True,
            bnb_4bit_quant_type="nf4",
            bnb_4bit_use_double_quant=True,
            bnb_4bit_compute_dtype=torch.float16,
        )

        tokenizer = AutoTokenizer.from_pretrained(self.cfg.base_model, use_fast=True)
        if tokenizer.pad_token is None:
            tokenizer.pad_token = tokenizer.eos_token

        base = AutoModelForCausalLM.from_pretrained(
            self.cfg.base_model,
            quantization_config=bnb,
            device_map="auto",
            torch_dtype=torch.float16,
        )
        base = prepare_model_for_kbit_training(base)

        lora_cfg = LoraConfig(
            r=16,
            lora_alpha=32,
            lora_dropout=0.05,
            bias="none",
            task_type="CAUSAL_LM",
            target_modules=["q_proj", "v_proj", "k_proj", "o_proj", "gate_proj", "up_proj", "down_proj"],
        )
        model = get_peft_model(base, lora_cfg)
        model.print_trainable_parameters()

        ds = load_dataset("json", data_files=str(dataset_path), split="train")

        def format_example(batch: dict) -> dict:
            texts = []
            for messages in batch["messages"]:
                pieces = []
                for m in messages:
                    pieces.append(f"<|{m['role']}|>\n{m['content']}\n")
                pieces.append("<|end|>")
                texts.append("".join(pieces))
            tokens = tokenizer(
                texts,
                truncation=True,
                max_length=2048,
                padding=False,
            )
            tokens["labels"] = tokens["input_ids"].copy()
            return tokens

        tokenized = ds.map(format_example, batched=True, remove_columns=ds.column_names)
        collator = DataCollatorForLanguageModeling(tokenizer=tokenizer, mlm=False)

        adapter_dir = Path(self.cfg.adapter_path)
        adapter_dir.mkdir(parents=True, exist_ok=True)
        train_args = TrainingArguments(
            output_dir=str(adapter_dir),
            num_train_epochs=self.cfg.epochs,
            per_device_train_batch_size=self.cfg.batch_size,
            gradient_accumulation_steps=self.cfg.grad_accum,
            learning_rate=self.cfg.learning_rate,
            warmup_ratio=self.cfg.warmup_ratio,
            fp16=True,
            logging_steps=10,
            save_strategy="epoch",
            save_total_limit=2,
            report_to=[],
        )
        trainer = Trainer(
            model=model,
            args=train_args,
            train_dataset=tokenized,
            data_collator=collator,
        )
        trainer.train()
        trainer.save_model(str(adapter_dir))
        tokenizer.save_pretrained(adapter_dir)
        manifest = {
            "base_model": self.cfg.base_model,
            "epochs": self.cfg.epochs,
            "train_examples": len(ds),
            "learning_rate": self.cfg.learning_rate,
        }
        (adapter_dir / "training_manifest.json").write_text(
            json.dumps(manifest, indent=2, ensure_ascii=False),
            encoding="utf-8",
        )
        return TrainArtifacts(
            adapter_dir=adapter_dir,
            train_examples=len(ds),
            epochs=self.cfg.epochs,
            base_model=self.cfg.base_model,
        )

    def merge_and_export(self, output_dir: str | Path) -> Path:
        from peft import PeftModel
        from transformers import AutoModelForCausalLM, AutoTokenizer

        output_dir = Path(output_dir)
        output_dir.mkdir(parents=True, exist_ok=True)
        base = AutoModelForCausalLM.from_pretrained(self.cfg.base_model)
        tokenizer = AutoTokenizer.from_pretrained(self.cfg.base_model)
        peft_model = PeftModel.from_pretrained(base, self.cfg.adapter_path)
        merged = peft_model.merge_and_unload()
        merged.save_pretrained(output_dir)
        tokenizer.save_pretrained(output_dir)
        log.info("merged model exported to %s", output_dir)
        return output_dir
