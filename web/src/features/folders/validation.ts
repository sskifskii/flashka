const edgeWhitespace = /^\p{White_Space}+|\p{White_Space}+$/gu;

export function normalizeFolderName(value: string) {
  return value.replace(edgeWhitespace, "");
}

export function validateFolderName(value: string) {
  const clean = normalizeFolderName(value);
  if ([...clean].length === 0) return "Поле обязательно";
  if ([...clean].length > 80) return "Не больше 80 символов";
  return "";
}
