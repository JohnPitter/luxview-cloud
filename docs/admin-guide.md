# Admin Guide

The admin panel is available to users with the `admin` role. Access it from the sidebar → **Admin**.

## Admin Panel Overview

The admin panel has four tabs:

### Users

- View all registered users (username, email, role, plan, GitHub info)
- **Change role** — promote or demote users (user ↔ admin)
- **Assign plan** — change a user's pricing plan

### Apps

- View all apps across all users
- **Force delete** — remove any app (bypasses ownership check)
- **Update limits** — change CPU/memory limits on any app

### VPS Info

- Real-time server information: CPU, memory, disk, uptime
- Docker container count and resource usage

### Plans

Manage pricing plans that control resource limits and feature access.

## Pricing Plans

Plans define what users can do on the platform. Each plan has:

### Resource Limits

| Field | Description | Example |
|---|---|---|
| Max Apps | Maximum number of apps a user can create | 3 |
| Max CPU per App | CPU cores allocated per app | 0.5 |
| Max Memory per App | Memory limit per app | 1g |
| Max Disk per App | Disk space per app | 2g |
| Max Services per App | Number of services per app | 3 |

### Feature Flags

| Flag | Description |
|---|---|
| Auto Deploy | Allow GitHub webhook auto-deploys |
| Custom Domain | Allow custom domain configuration |
| Priority Builds | Builds skip the queue |

### Display Settings

| Field | Description |
|---|---|
| Name | Plan display name (e.g., "Starter", "Pro") |
| Description | Short description shown to users |
| Price | Display price (no payment integration) |
| Currency | USD, BRL, EUR |
| Billing Cycle | monthly or yearly |
| Features | List of feature bullet points (shown on landing page) |
| Highlighted | Adds "Recommended" badge and visual emphasis |
| Sort Order | Controls display order on landing page |
| Is Default | Auto-assigned to new users (only one can be default) |

### Creating a Plan

1. Go to **Admin → Plans**
2. Click **Add Plan**
3. Fill in the form fields
4. Add feature bullet points (these appear on the landing pricing section)
5. Click **Save**

### Plan Enforcement

Plans are enforced at the API level:

- **Create App** — checks user's app count vs `max_apps`
- **Create Service** — checks app's service count vs `max_services_per_app`
- **Update App** — validates CPU/memory/disk vs plan limits
- **Auto Deploy** — checks `auto_deploy_enabled` flag
- **API response** — returns 403 with message: `"Plan limit reached: your {plan} plan allows max {limit} {resource}"`

### Default Plan

One plan can be marked as default. New users are automatically assigned this plan on registration. To change the default:

1. Go to **Admin → Plans**
2. Click **Set Default** on the desired plan

### Landing Page Pricing

The landing page pricing section (`/` → scroll to Pricing) dynamically renders all active plans:

- Plans are fetched from `GET /api/plans` (public, no auth required)
- Layout adapts to the number of plans (1-3 columns)
- Highlighted plans get an amber border and "Recommended" badge
- Plans are sorted by `sort_order`

## User Management

### Roles

| Role | Permissions |
|---|---|
| `user` | Create apps, deploy, manage own services |
| `admin` | Everything above + admin panel, user management, plan management |

### Assigning Plans to Users

1. Go to **Admin → Users**
2. Find the user
3. Click **Change Plan**
4. Select the new plan
5. Confirm

The change takes effect immediately. Existing apps are not affected, but new operations will be validated against the new plan limits.

## Internationalization

The platform supports three languages:

- English (en)
- Português (pt-BR)
- Español (es)

Language is auto-detected from the browser. Users can change it from **Settings → Language**.

All platform UI text uses translation keys. Plan names and descriptions are free text entered by the admin (not translated).
