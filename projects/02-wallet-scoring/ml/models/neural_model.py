from __future__ import annotations

import logging
from dataclasses import dataclass, field
from pathlib import Path

import numpy as np
import pandas as pd
import torch
from torch import nn
from torch.utils.data import DataLoader, Dataset

log = logging.getLogger(__name__)


class _WalletDataset(Dataset):
    def __init__(self, X: np.ndarray, y_score: np.ndarray | None = None, y_label: np.ndarray | None = None) -> None:
        self.X = torch.as_tensor(X, dtype=torch.float32)
        self.y_score = torch.as_tensor(y_score, dtype=torch.float32) if y_score is not None else None
        self.y_label = torch.as_tensor(y_label, dtype=torch.long) if y_label is not None else None

    def __len__(self) -> int:
        return self.X.shape[0]

    def __getitem__(self, idx: int):
        if self.y_score is None or self.y_label is None:
            return self.X[idx]
        return self.X[idx], self.y_score[idx], self.y_label[idx]


class _Network(nn.Module):
    def __init__(self, input_dim: int, num_classes: int = 3) -> None:
        super().__init__()
        self.backbone = nn.Sequential(
            nn.Linear(input_dim, 128),
            nn.BatchNorm1d(128),
            nn.ReLU(inplace=True),
            nn.Dropout(0.3),
            nn.Linear(128, 64),
            nn.BatchNorm1d(64),
            nn.ReLU(inplace=True),
            nn.Dropout(0.2),
            nn.Linear(64, 32),
            nn.ReLU(inplace=True),
        )
        self.score_head = nn.Linear(32, 1)
        self.cls_head = nn.Linear(32, num_classes)

    def forward(self, x: torch.Tensor) -> tuple[torch.Tensor, torch.Tensor]:
        z = self.backbone(x)
        score = self.score_head(z).squeeze(-1)
        logits = self.cls_head(z)
        return score, logits


@dataclass
class NeuralParams:
    epochs: int = 50
    batch_size: int = 256
    lr: float = 1e-3
    weight_decay: float = 1e-5
    score_weight: float = 0.7
    label_weight: float = 0.3
    patience: int = 10
    num_workers: int = 2
    device: str = "cpu"


@dataclass
class WalletNeuralModel:
    params: NeuralParams = field(default_factory=NeuralParams)
    net: _Network | None = None
    feature_names: list[str] | None = None
    input_dim: int = 0

    def train(
        self,
        X: pd.DataFrame,
        y_score: np.ndarray,
        y_label: np.ndarray,
        eval_set: tuple[pd.DataFrame, np.ndarray, np.ndarray] | None = None,
    ) -> dict[str, float]:
        self.feature_names = list(X.columns)
        self.input_dim = X.shape[1]
        device = torch.device(self.params.device)

        train_ds = _WalletDataset(X.to_numpy(dtype=np.float32),
                                  y_score.astype(np.float32),
                                  y_label.astype(np.int64))
        train_loader = DataLoader(
            train_ds,
            batch_size=self.params.batch_size,
            shuffle=True,
            num_workers=self.params.num_workers,
            drop_last=False,
        )

        val_loader: DataLoader | None = None
        if eval_set is not None:
            Xv, ys_val, yl_val = eval_set
            val_ds = _WalletDataset(Xv.to_numpy(dtype=np.float32),
                                    ys_val.astype(np.float32),
                                    yl_val.astype(np.int64))
            val_loader = DataLoader(val_ds, batch_size=self.params.batch_size, shuffle=False,
                                    num_workers=self.params.num_workers)

        net = _Network(self.input_dim).to(device)
        opt = torch.optim.Adam(net.parameters(), lr=self.params.lr,
                               weight_decay=self.params.weight_decay)
        mse = nn.MSELoss()
        ce = nn.CrossEntropyLoss()

        best_val = float("inf")
        epochs_since_improve = 0
        best_state = {k: v.detach().clone() for k, v in net.state_dict().items()}

        for epoch in range(self.params.epochs):
            net.train()
            total_loss = 0.0
            for xb, ys, yl in train_loader:
                xb = xb.to(device)
                ys = ys.to(device)
                yl = yl.to(device)
                opt.zero_grad()
                score_pred, logits = net(xb)
                loss = self.params.score_weight * mse(score_pred, ys) + \
                       self.params.label_weight * ce(logits, yl)
                loss.backward()
                opt.step()
                total_loss += float(loss.item()) * xb.size(0)
            train_loss = total_loss / len(train_ds)

            val_loss = self._evaluate(net, val_loader, mse, ce, device) if val_loader is not None else train_loss

            log.info("epoch %d train_loss=%.4f val_loss=%.4f", epoch + 1, train_loss, val_loss)

            if val_loss < best_val - 1e-4:
                best_val = val_loss
                epochs_since_improve = 0
                best_state = {k: v.detach().clone() for k, v in net.state_dict().items()}
            else:
                epochs_since_improve += 1
                if epochs_since_improve >= self.params.patience:
                    log.info("early stop at epoch %d", epoch + 1)
                    break

        net.load_state_dict(best_state)
        self.net = net.to(device)
        return {"best_val_loss": float(best_val)}

    def predict(self, X: pd.DataFrame) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
        self._require_trained()
        X_aligned = self._align(X)
        device = torch.device(self.params.device)
        self.net.eval()
        with torch.no_grad():
            xt = torch.as_tensor(X_aligned.to_numpy(dtype=np.float32), device=device)
            score, logits = self.net(xt)
            probs = torch.softmax(logits, dim=1).cpu().numpy()
            scores = score.cpu().numpy()
        labels = probs.argmax(axis=1).astype(int)
        return np.clip(scores, 0.0, 100.0), labels, probs

    def save(self, path: str | Path) -> None:
        self._require_trained()
        path = Path(path)
        path.parent.mkdir(parents=True, exist_ok=True)
        torch.save(
            {
                "state_dict": self.net.state_dict(),
                "feature_names": self.feature_names,
                "input_dim": self.input_dim,
                "params": self.params.__dict__,
            },
            path,
        )
        log.info("nn saved to %s", path)

    def load(self, path: str | Path) -> None:
        blob = torch.load(path, map_location=self.params.device)
        self.feature_names = blob["feature_names"]
        self.input_dim = blob["input_dim"]
        for k, v in blob.get("params", {}).items():
            setattr(self.params, k, v)
        net = _Network(self.input_dim)
        net.load_state_dict(blob["state_dict"])
        net.eval()
        self.net = net.to(self.params.device)
        log.info("nn loaded from %s", path)

    def _evaluate(self, net: _Network, loader: DataLoader | None, mse: nn.Module, ce: nn.Module, device: torch.device) -> float:
        if loader is None:
            return float("inf")
        net.eval()
        total_loss = 0.0
        seen = 0
        with torch.no_grad():
            for xb, ys, yl in loader:
                xb = xb.to(device)
                ys = ys.to(device)
                yl = yl.to(device)
                score_pred, logits = net(xb)
                loss = self.params.score_weight * mse(score_pred, ys) + \
                       self.params.label_weight * ce(logits, yl)
                total_loss += float(loss.item()) * xb.size(0)
                seen += xb.size(0)
        return total_loss / max(seen, 1)

    def _align(self, X: pd.DataFrame) -> pd.DataFrame:
        if self.feature_names is None:
            return X
        missing = [c for c in self.feature_names if c not in X.columns]
        for c in missing:
            X[c] = 0.0
        return X[self.feature_names]

    def _require_trained(self) -> None:
        if self.net is None:
            raise RuntimeError("neural model is not trained")
