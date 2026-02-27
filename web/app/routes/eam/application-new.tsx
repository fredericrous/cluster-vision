import { useState } from "react";
import { useNavigate } from "react-router";
import type { Route } from "./+types/application-new";
import styles from "./eam.module.css";

const API_URL = "/api";

export function meta({}: Route.MetaArgs) {
  return [{ title: "New Application — Cluster Vision EAM" }];
}

export async function loader() {
  return { isNew: true };
}

const statuses = ["active", "maintenance", "sunset", "retired"];
const criticalities = ["high", "medium", "low"];
const risks = ["high", "medium", "low"];
const phases = ["plan", "phase_in", "active", "phase_out", "end_of_life"];
const timeCategories = ["", "tolerate", "invest", "migrate", "eliminate"];

export default function ApplicationNew({ loaderData }: Route.ComponentProps) {
  const navigate = useNavigate();

  const [form, setForm] = useState({
    name: "",
    display_name: "",
    description: "",
    status: "active",
    business_criticality: "medium",
    technical_risk: "medium",
    lifecycle_phase: "active",
    time_category: "",
    end_of_life_date: "",
    tags: "",
  });

  const [saving, setSaving] = useState(false);

  function handleChange(e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) {
    setForm({ ...form, [e.target.name]: e.target.value });
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);

    try {
      const res = await fetch(`${API_URL}/eam/applications`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: form.name,
          display_name: form.display_name || null,
          description: form.description || null,
          status: form.status,
          business_criticality: form.business_criticality,
          technical_risk: form.technical_risk,
          lifecycle_phase: form.lifecycle_phase,
          time_category: form.time_category || null,
          end_of_life_date: form.end_of_life_date || null,
          tags: form.tags.split(",").map((t) => t.trim()).filter(Boolean),
        }),
      });
      if (!res.ok) throw new Error(await res.text());
      const app = await res.json();
      navigate(`/eam/applications/${app.id}`);
    } catch (err) {
      alert(`Failed to save: ${err}`);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className={styles.formPage}>
      <h1 className={styles.heading}>New Application</h1>

      <form onSubmit={handleSubmit} className={styles.formGrid}>
        <div className={styles.formRow}>
          <label className={styles.formLabel}>Name (unique identifier)</label>
          <input className={styles.formInput} name="name" value={form.name} onChange={handleChange} required />
        </div>

        <div className={styles.formRow}>
          <label className={styles.formLabel}>Display Name</label>
          <input className={styles.formInput} name="display_name" value={form.display_name} onChange={handleChange} />
        </div>

        <div className={styles.formRow}>
          <label className={styles.formLabel}>Description</label>
          <textarea className={styles.formInput} name="description" value={form.description} onChange={handleChange} rows={3} />
        </div>

        <div className={styles.formRow}>
          <label className={styles.formLabel}>Status</label>
          <select className={styles.formSelect} name="status" value={form.status} onChange={handleChange}>
            {statuses.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
        </div>

        <div className={styles.formRow}>
          <label className={styles.formLabel}>Business Criticality</label>
          <select className={styles.formSelect} name="business_criticality" value={form.business_criticality} onChange={handleChange}>
            {criticalities.map((c) => <option key={c} value={c}>{c}</option>)}
          </select>
        </div>

        <div className={styles.formRow}>
          <label className={styles.formLabel}>Technical Risk</label>
          <select className={styles.formSelect} name="technical_risk" value={form.technical_risk} onChange={handleChange}>
            {risks.map((r) => <option key={r} value={r}>{r}</option>)}
          </select>
        </div>

        <div className={styles.formRow}>
          <label className={styles.formLabel}>Lifecycle Phase</label>
          <select className={styles.formSelect} name="lifecycle_phase" value={form.lifecycle_phase} onChange={handleChange}>
            {phases.map((p) => <option key={p} value={p}>{p.replace("_", " ")}</option>)}
          </select>
        </div>

        <div className={styles.formRow}>
          <label className={styles.formLabel}>TIME Category</label>
          <select className={styles.formSelect} name="time_category" value={form.time_category} onChange={handleChange}>
            {timeCategories.map((t) => <option key={t} value={t}>{t || "— none —"}</option>)}
          </select>
        </div>

        <div className={styles.formRow}>
          <label className={styles.formLabel}>End of Life Date</label>
          <input className={styles.formInput} type="date" name="end_of_life_date" value={form.end_of_life_date} onChange={handleChange} />
        </div>

        <div className={styles.formRow}>
          <label className={styles.formLabel}>Tags (comma-separated)</label>
          <input className={styles.formInput} name="tags" value={form.tags} onChange={handleChange} />
        </div>

        <div className={styles.formActions}>
          <button type="submit" className={styles.btnPrimary} disabled={saving}>
            {saving ? "Saving..." : "Create"}
          </button>
          <button type="button" className={styles.btnSecondary} onClick={() => navigate(-1)}>
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
