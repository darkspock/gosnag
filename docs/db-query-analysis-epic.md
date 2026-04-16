# Epic: DB Query Analysis

## Product Thesis

**GoSnag ya recibe breadcrumbs SQL. Lo que falta es convertirlos en diagnóstico útil.**

Hoy un issue puede traer decenas o cientos de `db.query` breadcrumbs, pero la UI los muestra casi como texto crudo. Eso sirve para inspección manual, no para entender rápido:

- qué consultas se repiten
- cuánto tiempo consumen
- si hay un patrón N+1 dentro de una petición concreta
- si una query representativa tiene un plan de ejecución sospechoso

El objetivo de este epic es convertir los breadcrumbs SQL en una herramienta de triage y diagnóstico, sin transformar GoSnag en un cliente de base de datos generalista.

## Scope

Este epic cubre dos áreas:

1. **Observabilidad de SQL por issue**
   - mostrar mejor `db.query`
   - exponer `duration_ms`
   - agrupar queries repetidas
   - detectar N+1 bajo demanda

2. **Análisis activo contra la base de datos del proyecto**
   - conexión de análisis por proyecto
   - `Test connection`
   - `EXPLAIN` seguro y explícito sobre queries soportadas

## Relation with Existing N+1 Detection

GoSnag ya tiene detección batch de N+1 basada en `gosnag_query_summary`:

- extracción en [internal/n1/extract.go](/Users/juanmacias/Projects/GoSnag/internal/n1/extract.go:1)
- detector periódico en [internal/n1/detector.go](/Users/juanmacias/Projects/GoSnag/internal/n1/detector.go:1)

Este epic **no sustituye** ese sistema. Lo complementa:

- el detector batch sigue encontrando patrones repetidos entre múltiples requests
- el análisis por issue explica qué pasó en un request concreto

## Problem Statement

### Current UX Gap

En el detalle de un issue:

- los breadcrumbs `db.query` no se muestran como entidades SQL de primer nivel
- `duration_ms` puede existir en `breadcrumb.data`, pero no se explota bien en la UI
- no hay agrupación ni normalización de queries repetidas
- no hay una forma directa de pedir un análisis de N+1
- no hay `EXPLAIN` ni validación contra la BD real del proyecto

### Desired Outcome

Cuando un issue tenga breadcrumbs SQL, el usuario debe poder:

- ver claramente qué queries se ejecutaron y cuánto costó cada una
- detectar repeticiones y formas normalizadas
- lanzar un análisis que diga si hay señales de N+1
- ejecutar `EXPLAIN` sobre una query representativa usando una conexión segura del proyecto

## Architecture Overview

### A. SQL Breadcrumb Viewer

Nueva presentación especializada para breadcrumbs `db.query`:

- SQL visible y legible
- `duration_ms` destacado cuando exista
- agrupación por query normalizada
- métricas por grupo:
  - repeticiones
  - tiempo total
  - tiempo medio
  - disponibilidad de timing

### B. On-Demand Issue Analysis

Acción `Analyze` sobre un issue con SQL:

- recoge los `db.query` del último evento relevante
- normaliza las queries
- calcula señales heurísticas de N+1
- detecta si faltan tiempos
- opcionalmente ejecuta `EXPLAIN` sobre una query seleccionada

### C. Project-Level Analysis Connection

Cada proyecto puede tener una conexión específica para análisis:

- separada del DSN de ingest
- con permisos mínimos
- idealmente read-only
- nunca expuesta al frontend en claro

## Key Decisions

### 1. Separate Analysis Connection

La conexión de análisis debe ser independiente del DSN de ingest.

Razón:

- el DSN de ingest identifica el proyecto para recibir eventos
- no representa una conexión segura ni válida para ejecutar `EXPLAIN`

### 2. Timing First

Antes de análisis sofisticado, la UI debe exponer claramente `duration_ms`.

Razón:

- sin visibilidad de tiempos, el usuario no sabe si el problema es volumen, repetición o latencia
- la ausencia de timing es en sí una señal diagnóstica importante

### 3. EXPLAIN Must Be Explicit

`EXPLAIN` no debe ejecutarse automáticamente.

Razón:

- no toda query es apta
- no toda query es `SELECT`
- hay riesgo de meter accidentalmente SQL no seguro o demasiado caro

### 4. Heuristic N+1 is Acceptable

El análisis por issue puede ser heurístico.

Razón:

- un request concreto no siempre tiene la instrumentación perfecta
- el usuario necesita ayuda operativa, no prueba formal

## Data Model Direction

### Project Settings

Campos nuevos esperados en `projects` o configuración equivalente:

- `analysis_db_enabled`
- `analysis_db_driver`
- `analysis_db_dsn`
- opcionalmente:
  - `analysis_db_name`
  - `analysis_db_schema`
  - `analysis_db_notes`

### Optional Persisted Analysis

Se puede añadir persistencia de resultados de análisis en una segunda fase para:

- cachear análisis repetidos
- auditar qué se ejecutó
- evitar repetir `EXPLAIN` innecesariamente

No es obligatorio en la primera iteración.

## UX Direction

### Issue Detail

Nueva experiencia SQL dentro del detalle de issue:

- breadcrumbs SQL más legibles
- vista agrupada por query
- botón `Analyze`
- panel de resultados con:
  - resumen
  - grupos repetidos
  - hallazgos N+1
  - `EXPLAIN` cuando aplique
  - advertencias sobre timing ausente

### Project Settings

Nueva sección de configuración:

- `Database Analysis`
- estado de la conexión
- `Test connection`
- tipo de motor
- metadata no sensible visible

## API Direction

La API debe cubrir:

- guardar configuración de análisis de BD por proyecto
- probar conexión
- lanzar análisis por issue
- recuperar resultados de análisis si se cachean/persisten

Ejemplo de endpoint principal:

- `POST /api/v1/projects/{project_id}/issues/{issue_id}/db-analysis`

## Rollout

### Phase 1: Visibility + Heuristics

- mostrar `duration_ms`
- agrupar queries repetidas
- añadir conexión de análisis por proyecto
- `Test connection`
- análisis heurístico de N+1 por issue

### Phase 2: EXPLAIN

- `EXPLAIN` manual para `SELECT`
- representación estructurada del plan
- selección de query representativa

### Phase 3: Persistence and Smarts

- persistencia opcional del resultado
- soporte más rico por driver
- mejores heurísticas
- explicación asistida por IA como capa opcional, no base

## Success Criteria

El epic se considerará logrado cuando:

- un issue con SQL permita ver claramente tiempos y repeticiones
- el usuario pueda detectar rápidamente señales de N+1 en un request concreto
- exista un flujo seguro para ejecutar `EXPLAIN`
- la configuración sea por proyecto y operable desde la UI

## Out of Scope

- convertir GoSnag en una consola SQL general
- ejecutar SQL mutante
- hacer `EXPLAIN ANALYZE` automático
- depender de IA para el parser o la lógica base
