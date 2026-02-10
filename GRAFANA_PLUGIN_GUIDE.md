# Grafana Plugin Implementation Guide

## Overview

This guide provides a comprehensive implementation plan for a **PromSketch-specific Grafana datasource plugin**. While the standard Prometheus datasource works with PromSketch-Dropin, a custom plugin could provide sketch-specific features.

**Current Status**: Not implemented (Low priority - standard Prometheus plugin works well)

**Effort Estimate**: High (2-3 weeks for experienced Grafana plugin developer)

**Impact**: Low (standard Prometheus datasource provides 95% of needed functionality)

---

## Why a Custom Plugin?

### **Potential Benefits**

1. **Sketch-Specific Features**
   - Show sketch vs. backend routing decisions
   - Display sketch hit/miss metrics
   - Indicate query capability (sketch-supported or not)
   - Show sketch memory usage per series

2. **Enhanced Query Builder**
   - Autocomplete for sketch-supported functions
   - Warnings when queries won't use sketches
   - Sketch-specific query templates

3. **Sketch Metadata**
   - Browse available sketches
   - View sketch parameters (window size, k, kllK)
   - Display sketch coverage statistics

4. **Performance Insights**
   - Show execution time comparison (sketch vs. backend)
   - Display sketch compression ratios
   - Visualize sketch accuracy metrics

### **Why Low Priority?**

The standard Prometheus datasource already provides:
- ✅ Full query support (PromQL)
- ✅ Metric browser
- ✅ Label autocomplete
- ✅ Grafana Explore integration
- ✅ Alerting support
- ✅ Dashboard variables

---

## Implementation Prerequisites

### **Required Knowledge**

1. **TypeScript/JavaScript** - Plugin development language
2. **React** - Grafana UI framework
3. **Grafana Plugin SDK** - `@grafana/toolkit`, `@grafana/data`, `@grafana/ui`
4. **Prometheus API** - Understanding of Prometheus data model
5. **PromSketch Architecture** - Understanding of sketch capabilities

### **Development Environment**

```bash
# Required tools
- Node.js 18+ and npm 8+
- Grafana 9.0+ (or 10.0+)
- Docker (for testing)
- Git

# Recommended tools
- VS Code with Grafana plugin extensions
- Chrome DevTools
- Grafana's plugin validator
```

---

## Project Structure

```
promsketch-datasource/
├── src/
│   ├── datasource.ts          # Main datasource implementation
│   ├── query_editor.tsx       # Query editor component
│   ├── config_editor.tsx      # Datasource configuration
│   ├── variable_editor.tsx    # Variable query editor
│   ├── annotations_editor.tsx # Annotations support
│   ├── components/
│   │   ├── SketchMetrics.tsx  # Sketch-specific metrics display
│   │   ├── CapabilityBadge.tsx # Query capability indicator
│   │   └── SketchBrowser.tsx  # Browse available sketches
│   ├── api/
│   │   ├── prometheus.ts      # Prometheus API client
│   │   └── promsketch.ts      # PromSketch-specific APIs
│   ├── types.ts               # TypeScript type definitions
│   └── module.ts              # Plugin module definition
├── tests/
│   ├── datasource.test.ts
│   ├── query_editor.test.tsx
│   └── api.test.ts
├── plugin.json                # Plugin metadata
├── package.json               # Dependencies
├── tsconfig.json              # TypeScript configuration
├── .eslintrc                  # Linting rules
├── README.md                  # Plugin documentation
└── docker-compose.yml         # Test environment
```

---

## Implementation Steps

### **Phase 1: Project Setup (2-3 days)**

#### 1.1 Initialize Plugin Project

```bash
# Using Grafana's plugin generator
npx @grafana/create-plugin@latest

# Select options:
# - Plugin type: Datasource
# - Plugin name: PromSketch Datasource
# - Backend: No (use HTTP API)
```

#### 1.2 Update plugin.json

```json
{
  "type": "datasource",
  "name": "PromSketch",
  "id": "promsketch-datasource",
  "metrics": true,
  "alerting": true,
  "annotations": true,
  "logs": false,
  "backend": false,
  "executable": "",
  "info": {
    "description": "Datasource for PromSketch-Dropin with sketch-specific features",
    "author": {
      "name": "Your Name"
    },
    "keywords": ["promsketch", "prometheus", "sketches"],
    "version": "1.0.0",
    "updated": "2025-01-01"
  },
  "dependencies": {
    "grafanaDependency": ">=9.0.0",
    "plugins": []
  }
}
```

#### 1.3 Install Dependencies

```json
// package.json
{
  "dependencies": {
    "@grafana/data": "^10.0.0",
    "@grafana/runtime": "^10.0.0",
    "@grafana/ui": "^10.0.0",
    "react": "^18.0.0",
    "react-dom": "^18.0.0",
    "lodash": "^4.17.21"
  },
  "devDependencies": {
    "@grafana/toolkit": "^10.0.0",
    "@testing-library/react": "^14.0.0",
    "@types/react": "^18.0.0",
    "typescript": "^5.0.0",
    "eslint": "^8.0.0"
  }
}
```

### **Phase 2: Core Datasource Implementation (3-4 days)**

#### 2.1 Define Types

```typescript
// src/types.ts
import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface PromSketchQuery extends DataQuery {
  expr: string;           // PromQL expression
  format?: 'time_series' | 'table' | 'heatmap';
  instant?: boolean;
  range?: boolean;
  legendFormat?: string;
  interval?: string;
  intervalFactor?: number;
}

export interface PromSketchOptions extends DataSourceJsonData {
  url?: string;
  timeout?: number;
  httpMethod?: 'GET' | 'POST';
  showSketchMetrics?: boolean;  // NEW: Show sketch-specific metrics
  highlightSketchQueries?: boolean; // NEW: Highlight sketch-capable queries
}

export interface SketchCapability {
  canUseSketch: boolean;
  reason: string;
  requiredFunction?: string;
}

export interface SketchMetrics {
  sketchQueries: number;
  backendQueries: number;
  sketchHits: number;
  sketchMisses: number;
}
```

#### 2.2 Implement Datasource Class

```typescript
// src/datasource.ts
import {
  DataQueryRequest,
  DataQueryResponse,
  DataSourceApi,
  DataSourceInstanceSettings,
} from '@grafana/data';
import { getBackendSrv } from '@grafana/runtime';
import { PromSketchOptions, PromSketchQuery, SketchMetrics } from './types';

export class PromSketchDatasource extends DataSourceApi<PromSketchQuery, PromSketchOptions> {
  url: string;
  timeout: number;
  httpMethod: string;

  constructor(instanceSettings: DataSourceInstanceSettings<PromSketchOptions>) {
    super(instanceSettings);
    this.url = instanceSettings.url || '';
    this.timeout = instanceSettings.jsonData.timeout || 30000;
    this.httpMethod = instanceSettings.jsonData.httpMethod || 'POST';
  }

  async query(request: DataQueryRequest<PromSketchQuery>): Promise<DataQueryResponse> {
    const { range } = request;
    const promises = request.targets
      .filter(target => !target.hide)
      .map(target => this.performQuery(target, range));

    const results = await Promise.all(promises);

    return { data: results.flat() };
  }

  async performQuery(target: PromSketchQuery, range: any) {
    const url = target.instant
      ? `${this.url}/api/v1/query`
      : `${this.url}/api/v1/query_range`;

    const params: any = {
      query: target.expr,
    };

    if (!target.instant) {
      params.start = range.from.unix();
      params.end = range.to.unix();
      params.step = this.calculateStep(range, target);
    } else {
      params.time = range.to.unix();
    }

    const response = await getBackendSrv().datasourceRequest({
      url,
      method: this.httpMethod,
      data: this.httpMethod === 'POST' ? params : undefined,
      params: this.httpMethod === 'GET' ? params : undefined,
    });

    return this.processQueryResponse(response.data, target);
  }

  // NEW: Fetch sketch-specific metrics
  async getSketchMetrics(): Promise<SketchMetrics> {
    const response = await getBackendSrv().get(`${this.url}/metrics`);

    // Parse Prometheus metrics format
    const lines = response.split('\n');
    const metrics: SketchMetrics = {
      sketchQueries: 0,
      backendQueries: 0,
      sketchHits: 0,
      sketchMisses: 0,
    };

    lines.forEach(line => {
      if (line.startsWith('router_sketch_queries')) {
        metrics.sketchQueries = parseInt(line.split(' ')[1]);
      } else if (line.startsWith('router_backend_queries')) {
        metrics.backendQueries = parseInt(line.split(' ')[1]);
      } else if (line.startsWith('router_sketch_hits')) {
        metrics.sketchHits = parseInt(line.split(' ')[1]);
      } else if (line.startsWith('router_sketch_misses')) {
        metrics.sketchMisses = parseInt(line.split(' ')[1]);
      }
    });

    return metrics;
  }

  // NEW: Check if query can use sketches
  async checkSketchCapability(query: string): Promise<SketchCapability> {
    // Parse query to detect sketch-supported functions
    const sketchFunctions = ['avg_over_time', 'sum_over_time', 'count_over_time', 'quantile_over_time'];

    for (const func of sketchFunctions) {
      if (query.includes(func)) {
        return {
          canUseSketch: true,
          reason: 'Query uses sketch-supported function',
          requiredFunction: func,
        };
      }
    }

    return {
      canUseSketch: false,
      reason: 'Query does not use sketch-supported functions',
    };
  }

  async testDatasource() {
    const response = await getBackendSrv().get(`${this.url}/health`);

    if (response === 'OK\n' || response.status === 'OK') {
      return {
        status: 'success',
        message: 'PromSketch-Dropin is reachable',
      };
    }

    return {
      status: 'error',
      message: 'Failed to connect to PromSketch-Dropin',
    };
  }

  private processQueryResponse(data: any, target: PromSketchQuery) {
    // Convert Prometheus response to Grafana data frames
    // Implementation similar to standard Prometheus datasource
    // ...
  }

  private calculateStep(range: any, target: PromSketchQuery): string {
    // Calculate appropriate step for range query
    const intervalMs = range.to.diff(range.from);
    const step = Math.max(15, Math.floor(intervalMs / 1000 / 1000)); // 15s minimum
    return step + 's';
  }
}
```

### **Phase 3: Query Editor UI (3-4 days)**

#### 3.1 Sketch-Aware Query Editor

```typescript
// src/query_editor.tsx
import React, { useState, useEffect } from 'react';
import { QueryEditorProps } from '@grafana/data';
import { InlineField, Input, Select, Badge } from '@grafana/ui';
import { PromSketchDatasource } from './datasource';
import { PromSketchQuery, PromSketchOptions, SketchCapability } from './types';

export function QueryEditor(props: QueryEditorProps<PromSketchDatasource, PromSketchQuery, PromSketchOptions>) {
  const { query, onChange, onRunQuery, datasource } = props;
  const [capability, setCapability] = useState<SketchCapability | null>(null);

  // Check sketch capability when query changes
  useEffect(() => {
    if (query.expr) {
      datasource.checkSketchCapability(query.expr).then(setCapability);
    }
  }, [query.expr]);

  const onQueryChange = (value: string) => {
    onChange({ ...query, expr: value });
  };

  return (
    <div>
      <InlineField label="Query" labelWidth={20}>
        <Input
          value={query.expr || ''}
          onChange={(e) => onQueryChange(e.currentTarget.value)}
          onBlur={onRunQuery}
          placeholder="Enter PromQL query..."
          width={60}
        />
      </InlineField>

      {/* NEW: Sketch capability indicator */}
      {capability && (
        <InlineField label="Sketch Support">
          {capability.canUseSketch ? (
            <Badge text="✓ Can use sketches" color="green" />
          ) : (
            <Badge text="⚠ Backend only" color="orange" />
          )}
        </InlineField>
      )}

      {/* Additional query options */}
      <InlineField label="Format">
        <Select
          value={query.format || 'time_series'}
          options={[
            { label: 'Time series', value: 'time_series' },
            { label: 'Table', value: 'table' },
          ]}
          onChange={(v) => onChange({ ...query, format: v.value })}
        />
      </InlineField>
    </div>
  );
}
```

#### 3.2 Sketch Metrics Dashboard Component

```typescript
// src/components/SketchMetrics.tsx
import React, { useState, useEffect } from 'react';
import { Card, Stat, StatLabel, StatValue, HorizontalGroup } from '@grafana/ui';
import { PromSketchDatasource } from '../datasource';
import { SketchMetrics } from '../types';

interface Props {
  datasource: PromSketchDatasource;
}

export function SketchMetricsDisplay({ datasource }: Props) {
  const [metrics, setMetrics] = useState<SketchMetrics | null>(null);

  useEffect(() => {
    const loadMetrics = async () => {
      const m = await datasource.getSketchMetrics();
      setMetrics(m);
    };

    loadMetrics();
    const interval = setInterval(loadMetrics, 5000); // Refresh every 5s

    return () => clearInterval(interval);
  }, [datasource]);

  if (!metrics) {
    return <div>Loading metrics...</div>;
  }

  const hitRate = metrics.sketchQueries > 0
    ? ((metrics.sketchHits / metrics.sketchQueries) * 100).toFixed(1)
    : '0.0';

  return (
    <Card>
      <Card.Heading>Sketch Performance</Card.Heading>
      <Card.Content>
        <HorizontalGroup>
          <Stat>
            <StatLabel>Sketch Queries</StatLabel>
            <StatValue>{metrics.sketchQueries}</StatValue>
          </Stat>
          <Stat>
            <StatLabel>Backend Queries</StatLabel>
            <StatValue>{metrics.backendQueries}</StatValue>
          </Stat>
          <Stat>
            <StatLabel>Sketch Hits</StatLabel>
            <StatValue>{metrics.sketchHits}</StatValue>
          </Stat>
          <Stat>
            <StatLabel>Hit Rate</StatLabel>
            <StatValue>{hitRate}%</StatValue>
          </Stat>
        </HorizontalGroup>
      </Card.Content>
    </Card>
  );
}
```

### **Phase 4: Configuration Editor (1-2 days)**

```typescript
// src/config_editor.tsx
import React from 'react';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { InlineField, Input, Switch } from '@grafana/ui';
import { PromSketchOptions } from './types';

export function ConfigEditor(props: DataSourcePluginOptionsEditorProps<PromSketchOptions>) {
  const { options, onOptionsChange } = props;

  return (
    <div>
      <InlineField label="URL" labelWidth={20}>
        <Input
          value={options.jsonData.url || ''}
          onChange={(e) => onOptionsChange({
            ...options,
            jsonData: { ...options.jsonData, url: e.currentTarget.value },
          })}
          placeholder="http://localhost:9100"
          width={40}
        />
      </InlineField>

      <InlineField label="Timeout (ms)" labelWidth={20}>
        <Input
          type="number"
          value={options.jsonData.timeout || 30000}
          onChange={(e) => onOptionsChange({
            ...options,
            jsonData: { ...options.jsonData, timeout: parseInt(e.currentTarget.value, 10) },
          })}
          width={20}
        />
      </InlineField>

      {/* NEW: Sketch-specific options */}
      <InlineField label="Show Sketch Metrics" labelWidth={20}>
        <Switch
          value={options.jsonData.showSketchMetrics || false}
          onChange={(e) => onOptionsChange({
            ...options,
            jsonData: { ...options.jsonData, showSketchMetrics: e.currentTarget.checked },
          })}
        />
      </InlineField>

      <InlineField label="Highlight Sketch Queries" labelWidth={20}>
        <Switch
          value={options.jsonData.highlightSketchQueries || false}
          onChange={(e) => onOptionsChange({
            ...options,
            jsonData: { ...options.jsonData, highlightSketchQueries: e.currentTarget.checked },
          })}
        />
      </InlineField>
    </div>
  );
}
```

### **Phase 5: Testing (2-3 days)**

#### 5.1 Unit Tests

```typescript
// tests/datasource.test.ts
import { PromSketchDatasource } from '../src/datasource';

describe('PromSketchDatasource', () => {
  let datasource: PromSketchDatasource;

  beforeEach(() => {
    const instanceSettings = {
      url: 'http://localhost:9100',
      jsonData: {},
    } as any;
    datasource = new PromSketchDatasource(instanceSettings);
  });

  it('should check sketch capability correctly', async () => {
    const capability = await datasource.checkSketchCapability('avg_over_time(http_requests[5m])');
    expect(capability.canUseSketch).toBe(true);
    expect(capability.requiredFunction).toBe('avg_over_time');
  });

  it('should detect non-sketch queries', async () => {
    const capability = await datasource.checkSketchCapability('rate(http_requests[5m])');
    expect(capability.canUseSketch).toBe(false);
  });
});
```

### **Phase 6: Build & Distribution (1 day)**

```bash
# Build plugin
npm run build

# Sign plugin (requires Grafana signature)
npx @grafana/sign-plugin

# Package for distribution
zip -r promsketch-datasource.zip dist/
```

---

## Distribution Options

### **Option 1: Local Installation**

```bash
# Copy to Grafana plugins directory
cp -r dist/ /var/lib/grafana/plugins/promsketch-datasource/
systemctl restart grafana-server
```

### **Option 2: Grafana Plugin Catalog**

1. Create GitHub repository
2. Submit to Grafana plugin catalog
3. Complete Grafana review process
4. Plugin available for one-click install

### **Option 3: Docker**

```dockerfile
# Dockerfile for Grafana with PromSketch plugin
FROM grafana/grafana:latest
COPY dist/ /var/lib/grafana/plugins/promsketch-datasource/
```

---

## Alternative: Minimal Plugin

If full plugin is too much effort, create a **minimal plugin** that only adds:

1. **Sketch capability badge** in query editor
2. **Sketch metrics panel** in datasource settings

**Effort**: 2-3 days instead of 2-3 weeks

---

## Recommendation

**Use standard Prometheus datasource** unless you specifically need:
- Real-time sketch metrics in Grafana UI
- Query capability indicators
- Sketch-specific dashboards

The standard datasource provides 95% of functionality with zero development effort.

If custom features are needed later, start with the minimal plugin approach and expand based on user feedback.
