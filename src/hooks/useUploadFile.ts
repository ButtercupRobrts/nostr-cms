import { useMutation } from "@tanstack/react-query";
import { uploadMediaFile } from "@/lib/mediaProcessing";
import { useAppContext } from "./useAppContext";
import { useCurrentUser } from "./useCurrentUser";

export function useUploadFile() {
  const { config } = useAppContext();
  const { user } = useCurrentUser();

  return useMutation({
    mutationFn: async (file: File) => {
      if (!user) {
        throw new Error('Must be logged in to upload files');
      }

      const storedRelays = config.siteConfig?.blossomRelays || [];
      const excludedRelays = config.siteConfig?.excludedBlossomRelays || [];
      const defaultRelay = config.siteConfig?.defaultRelay;

      const relays = [...storedRelays];
      if (defaultRelay) {
        let normalizedDefault = defaultRelay.replace(/\/$/, '');
        if (normalizedDefault.startsWith('wss://')) {
          normalizedDefault = normalizedDefault.replace('wss://', 'https://');
        } else if (normalizedDefault.startsWith('ws://')) {
          normalizedDefault = normalizedDefault.replace('ws://', 'http://');
        }

        const isExcluded = excludedRelays.includes(normalizedDefault);

        if ((normalizedDefault.startsWith('http://') || normalizedDefault.startsWith('https://')) && !relays.includes(normalizedDefault) && !isExcluded) {
          relays.unshift(normalizedDefault);
        }
      }

      // Hard error when no Blossom server is configured — the previous
      // blossom.primal.net fallback would silently ship a user's media to a
      // third-party public server on any browser with an empty relay list.
      if (relays.length === 0) {
        throw new Error('No Blossom server configured');
      }

      const { tags } = await uploadMediaFile(
        file,
        relays,
        user.signer,
      );
      return tags;
    },
  });
}