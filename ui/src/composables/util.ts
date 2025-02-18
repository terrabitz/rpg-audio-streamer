export function patchObject<T extends object>(obj: T, patch: Partial<T>): T {
  return {
    ...obj,
    ...patch
  }
}