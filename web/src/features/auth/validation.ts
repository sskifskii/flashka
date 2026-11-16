import { z } from "zod";
export const credentialsSchema = z.object({
  login: z
    .string()
    .regex(
      /^[A-Za-z0-9][A-Za-z0-9_-]{2,31}$/,
      "Логин должен содержать 3–32 латинских символа, цифры, _ или -",
    ),
  password: z
    .string()
    .refine(
      (v) => Array.from(v).length >= 8 && Array.from(v).length <= 128,
      "Пароль должен содержать 8–128 символов",
    ),
});
export type Credentials = z.infer<typeof credentialsSchema>;
