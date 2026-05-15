import { defineStore } from "pinia";
import { getApiV1InviteByInviteCode, type InviteDetails } from "@/client/apiClient";
import { ref } from "vue";

export const useInviteStore = defineStore("invite", () => {
  const inviteDetails = ref<InviteDetails | null>(null);

  async function fetchInviteDetails(inviteCode: string) {
    try {
      const { data } = await getApiV1InviteByInviteCode<true>({
        path: { inviteCode },
      })

      inviteDetails.value = data;
    } catch (error) {
      console.error("Error fetching invite details:", error);
      inviteDetails.value = null;
    }
  }

  return {
    inviteDetails,
    fetchInviteDetails,
  };
});