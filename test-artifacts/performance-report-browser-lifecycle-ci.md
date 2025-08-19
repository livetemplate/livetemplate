# E2E Test Performance Report

**Test:** browser-lifecycle-ci
**Duration:** 1.307925s
**Success:** true
**Screenshots:** 0
**Errors:** 0

## Fragment Performance

| Fragment | Strategy | Generation Time | Size | Compression | Cache Hit |
|----------|----------|-----------------|------|-------------|----------|
| frag_static_dynamic_b10f5849fb1644f0 | static_dynamic | 1.262ms | 1657 bytes | 0.96% | ❌ |
| frag_static_dynamic_fd589d55fb3945e3 | static_dynamic | 925.458µs | 1632 bytes | 100.00% | ❌ |
| frag_granular_8b026c1690fb0290 | granular | 180.25µs | 1749 bytes | 100.00% | ❌ |
| frag_replacement_5d15cc3686709b4e | replacement | 492.125µs | 1133 bytes | 76.44% | ❌ |
| frag_replacement_e62db61b7fe6795a | replacement | 1.148292ms | 1058 bytes | 75.83% | ❌ |
| frag_replacement_7f134d485d9102cb | replacement | 266.083µs | 1147 bytes | 76.44% | ❌ |
| frag_replacement_d8fe6c613507aa85 | replacement | 199.583µs | 1058 bytes | 75.83% | ❌ |
| frag_replacement_b9340a12196e7a0a | replacement | 202.833µs | 1147 bytes | 76.44% | ❌ |

## Browser Actions

| Action | Duration | Status | Error |
|--------|----------|-----------|-------|
| initial-page-load | 1.096083042s | ✅ | - |
| validate-initial-content | 1.458791ms | ✅ | - |
| fragment-update-text-only-update | 4.719167ms | ✅ | - |
| fragment-update-attribute-update | 16.498708ms | ✅ | - |
| fragment-update-structural-update | 16.069917ms | ✅ | - |
| rapid-updates-5x | 83.429333ms | ✅ | - |
| final-validation | 1.508583ms | ✅ | - |

## Custom Metrics

- **fragments_attribute-update_count**: 1
- **strategy_structural-update_found**: true
- **server_url**: http://127.0.0.1:65487
- **fragments_text-only-update_count**: 1
- **strategy_text-only-update_found**: true
- **strategy_attribute-update_found**: false
- **fragments_structural-update_count**: 1
- **rapid_updates_duration**: 83.430333ms
- **final_title**: LiveTemplate CI Test
- **initial_header_text**: LiveTemplate CI Test
- **initial_count**: 42

