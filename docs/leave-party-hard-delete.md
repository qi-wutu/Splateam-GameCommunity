---
name: leave-party-hard-delete
description: LeaveParty 使用 Unscoped 硬删的原因和影响
metadata:
  type: project
  tags: [bug, party, architecture]
---

## LeaveParty 硬删问题

**现象：** 退出（Leave）后无法再次加入同一个队伍，报 `Duplicate entry` 错误。

**原因：**
`PartyMember` 表有 `uniqueIndex:idx_party_user`（`party_id` + `user_id` 联合唯一索引）。
如果 Leave 时用 `Delete()`（软删），记录仍然在 DB 里带着 `deleted_at` 值，
再次 Join 时 `INSERT` 会因唯一索引冲突而失败。

**修复：**
LeaveParty 使用 `Unscoped().Delete()` 硬删成员记录，允许用户退出后重新加入。

**为什么硬删可以接受：**
- 退队记录不需要保留（没有审计需求）
- Owner 删除整队时，成员记录也走软删（保持一致性，可恢复）
- 唯一索引保证了不会出现重复的活跃成员
- 硬删后 `RuserinP` 查不到，Join 可以正常 INSERT

**相关文件：**
- [[party-service]] — `LeaveParty()` 中的 `Unscoped()`
- [[party-models]] — `PartyMember` 的 `uniqueIndex:idx_party_user`

**为什么评价为 Bug：**
设计上用户退出后应该能重新加入，但软删 + 唯一索引的组合使得退出后无法再加。违反了基本的产品预期。
