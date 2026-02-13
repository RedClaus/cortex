---
project: Cortex
component: Unknown
phase: Design
date_created: 2026-02-04T00:00:00
source: ServerProjectsMac
librarian_indexed: 2026-02-06T01:16:29.749901
---

# Evaluation: Unbrowse for OpenClaw — Swarm Adoption Analysis

**Date:** 2026-02-04  
**Repository:** https://github.com/lekt9/unbrowse-openclaw  
**Evaluator:** Albert  
**Version:** 0.5.0  
**License:** UNLICENSED (proprietary)  

---

## Executive Summary

| Aspect | Assessment |
|--------|------------|
| **Adopt for Swarm** | ⚠️ **Conditional** — Powerful but has concerns |
| **Adopt for All Agents** | ❌ Not recommended (financial/security risks) |
| **Integration Effort** | Low-Medium (OpenClaw plugin) |
| **Value Proposition** | High — 100x faster web access via API reverse engineering |
| **Blockers** | x402 crypto payments, proprietary license, external service dependency |

**Verdict:** Evaluate in isolated testing. Do NOT deploy to production swarm without addressing financial/security concerns.

---

## What Unbrowse Does

**Core Value Proposition:**
- Captures internal APIs from any website during normal browsing
- Auto-generates OpenClaw skills that call APIs directly (200ms vs 12s browser automation)
- Marketplace for sharing/purchasing API skills
- x402 protocol for machine-to-machine payments (Solana/USDC)

**Workflow:**
```
1. Browse website normally
2. Unbrowse records all internal API calls (endpoints, auth, formats)
3. Auto-generates skill: website.getData(), website.postAction()
4. Agent uses skill → direct API calls (100x faster)
```

**Example:**
```typescript
// Auto-generated from browsing Polymarket
polymarket.getMarkets()      // 200ms instead of 12s browser automation
polymarket.getOdds(marketId) // 150ms
polymarket.placeBet(...)     // 180ms
```

---

## Integration with OpenClaw

### Plugin Architecture
- **Type:** OpenClaw native plugin (`openclaw.plugin.json`)
- **Entry:** `dist/index.js`
- **Hooks:** `hooks/auto-discover` — automatically generates skills while browsing
- **Config:** Browser port, auto-discover toggle, marketplace URL, wallet settings

### Configuration Options
```json
{
  "skillsOutputDir": "~/.openclaw/skills",
  "browserPort": 18791,
  "autoDiscover": true,
  "skillIndexUrl": "https://index.unbrowse.ai",
  "marketplace": {
    "creatorWallet": "SOLANA_ADDRESS",
    "solanaPrivateKey": "BASE58_KEY",
    "defaultPrice": "0"
  },
  "browser": {
    "useApiKey": "bu_...",
    "proxyCountry": "us"
  }
}
```

---

## 🟢 Strong Cases FOR Adoption

### 1. **Dramatic Speed Improvement**
| Operation | Browser Automation | Unbrowse API | Speedup |
|-----------|-------------------|--------------|---------|
| Check odds | 12s | 200ms | **60x** |
| Scrape data | 45s | 300ms | **150x** |
| Form submit | 8s | 180ms | **44x** |

### 2. **Fits Swarm Architecture**
- OpenClaw plugin = native integration
- Auto-discovery hook requires no code changes
- Skills output to standard directory
- Compatible with A2A bridge skill sharing

### 3. **Skill Marketplace Value**
- Agents can download pre-built skills ("Google for agents")
- Swarm could publish internal tools as skills
- Network effects: more users = more skills

### 4. **Real-World Problem Solved**
- 99% of sites lack official APIs
- MCP servers are manually built (dozens exist, millions of sites)
- Browser automation is fragile and slow

---

## 🔴 Major Concerns AGAINST Adoption

### 1. **x402 Crypto Payments (FINANCIAL REDLINE)** ⚠️ CRITICAL
```json
"marketplace": {
  "creatorWallet": "SOLANA_ADDRESS",      // Receives USDC
  "solanaPrivateKey": "BASE58_KEY",       // Spends USDC
  "defaultPrice": "2.50"                  // USD per skill
}
```

**Issues:**
- Requires Solana wallet with private keys
- Automatic USDC spending for paid skills
- **Violates your financial guardrails** — agents making crypto payments
- No human approval loop shown in docs

**Your Policy:** "Strictly forbidden from accessing bank accounts, crypto wallets, or any financial interfaces unless explicit per-session mandate"

### 2. **Proprietary License (UNLICENSED)**
- Not open source
- No self-hosting option for marketplace
- Dependency on `index.unbrowse.ai` (external service)
- Vendor lock-in risk

### 3. **Security & Privacy Risks**
- Records ALL browser traffic (including sensitive sites)
- Captures auth tokens, cookies, headers
- Potential credential leakage in generated skills
- Auto-login integration with keychain/1Password

### 4. **External Service Dependency**
- Cloud marketplace required for skill discovery
- Browser-use.com API for stealth browsers
- Unbrowse.ai infrastructure (startup risk)
- LAN/offline operation not possible

### 5. **Terms of Service Violations**
- Reverse engineering website APIs may violate ToS
- Scraping at scale could trigger legal issues
- No rate limiting or ethics framework shown

---

## Alignment with Swarm Goals

| Swarm Need | Unbrowse Fit | Notes |
|------------|--------------|-------|
| **Fast web access** | ✅ Excellent | 100x speedup |
| **Tool ecosystem** | ✅ Good | Marketplace model |
| **Self-hosted** | ❌ Poor | Cloud-dependent |
| **Privacy-first** | ⚠️ Risky | Records all traffic |
| **Financial safety** | ❌ Violates | x402 payments |
| **Open source** | ❌ No | Proprietary |
| **Offline capable** | ❌ No | Requires cloud |

---

## Alternative Approaches

### Option A: Use Without Marketplace (Recommended for Testing)
```json
{
  "autoDiscover": true,
  "marketplace": { "defaultPrice": "0" },  // Free only
  "skillIndexUrl": null                      // Disable marketplace
}
```
- Keep API discovery
- Disable payments entirely
- Skills stay local-only
- Still has external dependency issues

### Option B: Manual API Documentation
Instead of auto-discovery:
1. Manually document internal APIs when needed
2. Write custom OpenClaw skills
3. Store in swarm GitHub repos
4. Share via A2A bridge

**Pros:** Full control, no crypto, no external deps  
**Cons:** Manual work, slower than auto-discovery

### Option C: Build Open-Source Alternative
Create a self-hosted version:
- MIT license
- No marketplace/payments
- Local-only API discovery
- Swarm-shared skill repository

**Effort:** High (weeks)  
**Value:** Full control, aligns with swarm principles

---

## Specific Risks for Each Agent

| Agent | Risk Level | Concern |
|-------|------------|---------|
| **Albert (you)** | 🔴 High | Financial redline, policy violation |
| **Harold** | 🟡 Medium | Could leak internal network APIs |
| **Pink** | 🟡 Medium | External deps break LAN isolation |
| **Red** | 🟡 Medium | Same as Pink |
| **Kentaro** | 🟢 Low | Isolated, less sensitive |

---

## Recommendation

### 🟡 **Conditional Adoption — Testing Phase Only**

**DO:**
- ✅ Install in isolated test environment
- ✅ Use only for public, non-sensitive sites
- ✅ Disable marketplace (`skillIndexUrl: null`)
- ✅ Disable payments (no wallet config)
- ✅ Audit all generated skills before use
- ✅ Document any valuable APIs found

**DO NOT:**
- ❌ Deploy to production swarm
- ❌ Enable crypto payments
- ❌ Use on sensitive/internal sites
- ❌ Allow auto-publishing to marketplace
- ❌ Integrate with keychain/1Password

### Next Steps

1. **Week 1:** Install in test OpenClaw instance
2. **Week 2:** Browse 5-10 common sites, capture APIs
3. **Week 3:** Evaluate generated skill quality
4. **Week 4:** Decision — adopt, modify, or reject

---

## Comparison: Unbrowse vs Current Approach

| Capability | Current (Browser Tools) | Unbrowse | Winner |
|------------|------------------------|----------|--------|
| **Speed** | 10-45s | 200ms | Unbrowse |
| **Reliability** | Fragile (DOM selectors) | Robust (API contracts) | Unbrowse |
| **Setup** | Works everywhere | Plugin install | Current |
| **Privacy** | Local only | Cloud-dependent | Current |
| **Cost** | Free | Crypto payments | Current |
| **Control** | Full | Vendor-dependent | Current |

---

## Final Verdict

**Unbrowse is powerful but dangerous.**

The API reverse engineering is genuinely valuable — 100x speedups are compelling. But the crypto payment integration, proprietary license, and external dependencies create unacceptable risks for a production swarm.

**Recommended path:**
1. Test locally with payments disabled
2. If valuable, consider building open-source alternative
3. Never enable x402 payments without explicit per-session mandate
4. Document findings for future reference

---

**References:**
- Repository: https://github.com/lekt9/unbrowse-openclaw
- Website: https://unbrowse.ai
- x402 Protocol: https://x402.org
- OpenClaw Plugin Spec: `openclaw.plugin.json` in repo

**Related Vault Files:**
- `Cortex/TODO-Cortex-Avatar.md` — Similar tool ecosystem evaluation
- `Cortex/Evaluations/UI-TARS-desktop-Evaluation.md` — Comparable pattern

---

*Evaluation Status: COMPLETE*  
*Next Review: After 4-week testing phase (if pursued)*
