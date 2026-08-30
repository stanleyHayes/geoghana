import { FairUseConsole } from "@/components/fair-use-console";
import { RequirePermission } from "@/components/session";

export default function Screen() {
  return <RequirePermission permission="fairuse:manage"><FairUseConsole /></RequirePermission>;
}
