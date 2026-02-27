CREATE TYPE app_status AS ENUM ('active', 'maintenance', 'sunset', 'retired');
CREATE TYPE criticality AS ENUM ('high', 'medium', 'low');
CREATE TYPE risk_level AS ENUM ('high', 'medium', 'low');
CREATE TYPE lifecycle_phase AS ENUM ('plan', 'phase_in', 'active', 'phase_out', 'end_of_life');
CREATE TYPE it_component_type AS ENUM ('database', 'messaging', 'storage', 'compute', 'network', 'runtime', 'observability', 'security');
CREATE TYPE time_category AS ENUM ('tolerate', 'invest', 'migrate', 'eliminate');
CREATE TYPE data_source AS ENUM ('auto-discovered', 'ai-inferred', 'human-verified');

CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    display_name TEXT,
    description TEXT,
    description_source data_source DEFAULT 'auto-discovered',
    status app_status NOT NULL DEFAULT 'active',
    business_criticality criticality DEFAULT 'medium',
    business_criticality_source data_source DEFAULT 'auto-discovered',
    technical_risk risk_level DEFAULT 'medium',
    technical_risk_source data_source DEFAULT 'auto-discovered',
    technical_risk_reasoning TEXT,
    lifecycle_phase lifecycle_phase DEFAULT 'active',
    time_category time_category,
    time_category_source data_source DEFAULT 'auto-discovered',
    time_category_reasoning TEXT,
    end_of_life_date DATE,
    tags TEXT[] DEFAULT '{}',
    ai_confidence REAL DEFAULT 0,
    manual_override BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE it_components (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    type it_component_type NOT NULL,
    version TEXT,
    provider TEXT,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, type)
);

CREATE TABLE business_capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    description TEXT,
    parent_id UUID REFERENCES business_capabilities(id) ON DELETE SET NULL,
    level INT NOT NULL DEFAULT 1,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE app_dependencies (
    source_app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    target_app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    description TEXT,
    PRIMARY KEY (source_app_id, target_app_id),
    CHECK(source_app_id != target_app_id)
);

CREATE TABLE app_components (
    app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    component_id UUID NOT NULL REFERENCES it_components(id) ON DELETE CASCADE,
    PRIMARY KEY (app_id, component_id)
);

CREATE TABLE app_capabilities (
    app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    capability_id UUID NOT NULL REFERENCES business_capabilities(id) ON DELETE CASCADE,
    PRIMARY KEY (app_id, capability_id)
);

CREATE TABLE k8s_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    cluster TEXT NOT NULL,
    namespace TEXT NOT NULL,
    helm_release TEXT,
    workload_name TEXT,
    workload_kind TEXT,
    chart_name TEXT,
    chart_version TEXT,
    images TEXT[] DEFAULT '{}',
    last_sync_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    manual_override BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE version_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    chart_version TEXT,
    image_tag TEXT,
    latest_version TEXT,
    outdated BOOLEAN DEFAULT false,
    vuln_critical INT DEFAULT 0,
    vuln_high INT DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_version_history_app_time ON version_history(app_id, recorded_at DESC);

CREATE TABLE sync_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    apps_created INT DEFAULT 0,
    apps_updated INT DEFAULT 0,
    components_created INT DEFAULT 0,
    errors TEXT[] DEFAULT '{}',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);
