import { useMutation } from "@tanstack/react-query";
import { BlossomUploader } from '@nostrify/nostrify/uploaders';
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

      const uploader = new BlossomUploader({
        servers: config.siteConfig?.blossomRelays || [
          'https://blossom.primal.net/',
        ],
        signer: user.signer,
      });

      const tags = await uploader.upload(file);
      return tags;
    },
  });
}