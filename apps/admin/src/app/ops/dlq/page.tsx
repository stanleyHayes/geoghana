import { PlannedScreen } from "@/components/screen";
import { SCREENS } from "@/config/screens";

export default function Screen() {
  return <PlannedScreen {...SCREENS["/ops/dlq"]!} />;
}
