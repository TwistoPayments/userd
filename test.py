from pprint import pprint
import os.path
from googleapiclient.discovery import build
from google_auth_oauthlib.flow import InstalledAppFlow
from google.auth.transport.requests import Request

from oauth2client.service_account import ServiceAccountCredentials


# If modifying these scopes, delete the file token.pickle.
SCOPES = [
    'https://www.googleapis.com/auth/admin.directory.group.readonly',
    'https://www.googleapis.com/auth/admin.directory.user.readonly',
    'https://www.googleapis.com/auth/admin.directory.userschema.readonly',
    'https://www.googleapis.com/auth/admin.directory.user.readonly',
]


def main():
    """Shows basic usage of the Admin SDK Directory API.
    Prints the emails and names of the first 10 users in the domain.
    """
    credentials = ServiceAccountCredentials.from_json_keyfile_name(os.environ['GOOGLE_APPLICATION_CREDENTIALS'], scopes=SCOPES)
    credentials = credentials.create_delegated(os.environ['GOOGLE_ADMIN_SUBJECT'])
    service = build('admin', 'directory_v1', credentials=credentials)

    # Call the Admin SDK Directory API
    req = service.users().get(userKey='filip.sedlak@twisto.cz', projection='full')
    pprint(req.execute())

    # req = service.users().list(customer='C03akk3u5')
    # results = req.execute()
    # users = results.get('users', [])

    # while True:
    #     req = service.users().list_next(req, results)
    #     if not req:
    #         break
    #     results = req.execute()
    #     users.extend(results.get('users', []))

    # for user in users:
    #     print(u'{0} ({1})'.format(user['primaryEmail'],
    #         user['name']['fullName']))
    #     pprint(user)


if __name__ == '__main__':
    main()
