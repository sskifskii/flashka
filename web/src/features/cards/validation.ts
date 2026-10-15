export function normalizeSide(value: string) {
  return value.replace(/^\p{White_Space}+|\p{White_Space}+$/gu, "");
}
export function validateSide(value: string) {
  const normalized = normalizeSide(value);
  if ([...normalized].length === 0) return "Поле обязательно";
  if ([...normalized].length > 2000) return "Не больше 2000 символов";
  return "";
}
