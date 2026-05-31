from __future__ import annotations

import logging
from dataclasses import dataclass, field
from pathlib import Path

import joblib
import numpy as np
import torch
from sklearn.preprocessing import StandardScaler
from torch import nn
from torch.utils.data import DataLoader, Dataset

log = logging.getLogger(__name__)


LSTM_FEATURE_COLUMNS: list[str] = [
    "ofi",
    "vpin",
    "price_impact",
    "avg_swap_size",
    "buy_ratio",
    "cumulative_volume",
    "price_range",
    "price_close",
    "swap_count",
]


class SequenceDataset(Dataset):
    def __init__(self, sequences: np.ndarray, labels: np.ndarray | None = None) -> None:
        self.sequences = torch.as_tensor(sequences, dtype=torch.float32)
        self.labels = torch.as_tensor(labels, dtype=torch.long) if labels is not None else None

    def __len__(self) -> int:
        return self.sequences.shape[0]

    def __getitem__(self, idx: int):
        if self.labels is None:
            return self.sequences[idx]
        return self.sequences[idx], self.labels[idx]


class _Net(nn.Module):
    def __init__(self, input_size: int, hidden: int, layers: int, dropout: float, num_classes: int = 3) -> None:
        super().__init__()
        self.lstm = nn.LSTM(
            input_size=input_size,
            hidden_size=hidden,
            num_layers=layers,
            dropout=dropout if layers > 1 else 0.0,
            batch_first=True,
        )
        self.head = nn.Sequential(
            nn.Linear(hidden, 32),
            nn.ReLU(inplace=True),
            nn.Linear(32, num_classes),
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        out, _ = self.lstm(x)
        last = out[:, -1, :]
        return self.head(last)


@dataclass
class LSTMParams:
    seq_len: int = 12
    input_size: int = len(LSTM_FEATURE_COLUMNS)
    hidden: int = 64
    layers: int = 2
    dropout: float = 0.2
    epochs: int = 80
    batch_size: int = 64
    lr: float = 1e-3
    patience: int = 10
    num_workers: int = 0
    random_state: int = 42
    device: str = "cpu"


@dataclass
class DirectionLSTM:
    params: LSTMParams = field(default_factory=LSTMParams)
    net: _Net | None = None
    scaler: StandardScaler | None = None
    feature_columns: list[str] = field(default_factory=lambda: list(LSTM_FEATURE_COLUMNS))

    def fit(self, sequences: np.ndarray, labels: np.ndarray, val_split: float = 0.15) -> dict[str, float]:
        if sequences.size == 0:
            raise ValueError("empty sequences")
        torch.manual_seed(self.params.random_state)
        device = torch.device(self.params.device)

        n, seq_len, dim = sequences.shape
        self.params.seq_len = seq_len
        self.params.input_size = dim

        val_count = max(1, int(n * val_split))
        train_raw = sequences[:-val_count]
        val_raw = sequences[-val_count:]
        train_lbl, val_lbl = labels[:-val_count], labels[-val_count:]

        self.scaler = StandardScaler()
        self.scaler.fit(train_raw.reshape(-1, dim))
        train_seq = self.scaler.transform(train_raw.reshape(-1, dim)).reshape(train_raw.shape).astype(np.float32)
        val_seq = self.scaler.transform(val_raw.reshape(-1, dim)).reshape(val_raw.shape).astype(np.float32)

        train_loader = DataLoader(
            SequenceDataset(train_seq, train_lbl),
            batch_size=self.params.batch_size,
            shuffle=True,
            num_workers=self.params.num_workers,
        )
        val_loader = DataLoader(
            SequenceDataset(val_seq, val_lbl),
            batch_size=self.params.batch_size,
            shuffle=False,
            num_workers=self.params.num_workers,
        )

        self.net = _Net(dim, self.params.hidden, self.params.layers, self.params.dropout).to(device)
        opt = torch.optim.Adam(self.net.parameters(), lr=self.params.lr)
        criterion = nn.CrossEntropyLoss()

        best_val = float("inf")
        patience_left = self.params.patience
        best_state = {k: v.detach().clone() for k, v in self.net.state_dict().items()}

        for epoch in range(self.params.epochs):
            self.net.train()
            running = 0.0
            seen = 0
            for xb, yb in train_loader:
                xb = xb.to(device)
                yb = yb.to(device)
                opt.zero_grad()
                logits = self.net(xb)
                loss = criterion(logits, yb)
                loss.backward()
                opt.step()
                running += float(loss.item()) * xb.size(0)
                seen += xb.size(0)
            train_loss = running / max(1, seen)
            val_loss = self._eval_loss(val_loader, criterion, device)
            log.info("lstm epoch=%d train=%.4f val=%.4f", epoch + 1, train_loss, val_loss)
            if val_loss < best_val - 1e-4:
                best_val = val_loss
                patience_left = self.params.patience
                best_state = {k: v.detach().clone() for k, v in self.net.state_dict().items()}
            else:
                patience_left -= 1
                if patience_left <= 0:
                    log.info("lstm early stop epoch=%d", epoch + 1)
                    break

        self.net.load_state_dict(best_state)
        self.net.eval()
        return {"best_val_loss": float(best_val)}

    def predict_proba(self, sequences: np.ndarray) -> np.ndarray:
        self._require_trained()
        device = torch.device(self.params.device)
        n, seq_len, dim = sequences.shape
        flat = sequences.reshape(-1, dim)
        scaled = self.scaler.transform(flat).reshape(n, seq_len, dim).astype(np.float32)
        self.net.eval()
        with torch.no_grad():
            xt = torch.as_tensor(scaled, device=device)
            logits = self.net(xt)
            probs = torch.softmax(logits, dim=1).cpu().numpy()
        return probs

    def _eval_loss(self, loader: DataLoader, criterion: nn.Module, device: torch.device) -> float:
        self.net.eval()
        total = 0.0
        seen = 0
        with torch.no_grad():
            for xb, yb in loader:
                xb = xb.to(device)
                yb = yb.to(device)
                logits = self.net(xb)
                loss = criterion(logits, yb)
                total += float(loss.item()) * xb.size(0)
                seen += xb.size(0)
        return total / max(1, seen)

    def save(self, dir_path: str | Path) -> None:
        self._require_trained()
        dir_path = Path(dir_path)
        dir_path.mkdir(parents=True, exist_ok=True)
        torch.save(self.net.state_dict(), dir_path / "lstm.pt")
        joblib.dump({
            "scaler": self.scaler,
            "params": self.params,
            "feature_columns": self.feature_columns,
        }, dir_path / "lstm_meta.joblib")

    def load(self, dir_path: str | Path) -> None:
        dir_path = Path(dir_path)
        meta = joblib.load(dir_path / "lstm_meta.joblib")
        self.scaler = meta["scaler"]
        self.params = meta.get("params", self.params)
        self.feature_columns = meta.get("feature_columns", self.feature_columns)
        net = _Net(self.params.input_size, self.params.hidden, self.params.layers, self.params.dropout)
        net.load_state_dict(torch.load(dir_path / "lstm.pt", map_location=self.params.device))
        net.eval()
        self.net = net.to(self.params.device)

    def _require_trained(self) -> None:
        if self.net is None or self.scaler is None:
            raise RuntimeError("lstm is not trained")
